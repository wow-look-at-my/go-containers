// Package concurrentmap provides a generic map for concurrent use. The map
// shards its keys across independently locked partitions, so operations on
// different shards never contend.
//
// The API follows .NET's ConcurrentDictionary. It differs on one point, and
// the difference is deliberate: [Map.LoadOrCompute], [Map.AddOrUpdate] and
// [Map.Compute] run the caller's function while the shard lock is held.
package concurrentmap

import (
	"fmt"
	"hash/maphash"
	"iter"
	"math/bits"
	"runtime"
	"sync"
	"unsafe"
)

// Shard-count bounds: the floor stops a small machine serializing on one lock; the ceiling keeps Len cheap.
const (
	minShards = 8
	maxShards = 1024
)

// errZeroMap names the fix instead of panicking with a bare nil-map trace.
const errZeroMap = "concurrentmap: the zero Map is not usable; call New"

// shardBytes covers a 64-byte cache line plus the adjacent-line prefetch on amd64, so two shards never share one.
const shardBytes = 128

// padBytes fills the rest: a map value is one pointer whatever K/V are.
const padBytes = shardBytes - unsafe.Sizeof(sync.RWMutex{}) - unsafe.Sizeof(map[int]int(nil))

// shard is one independently locked partition of a [Map]. Each shard is its
// own allocation, so the padding survives.
type shard[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
	_  [padBytes]byte
}

// pair is one key and its value in an iteration snapshot.
type pair[K comparable, V any] struct {
	key   K
	value V
}

// Map is safe for concurrent use: keys spread across shards, each its own
// lock. Zero value NOT usable (every method panics) -- create with [New], and
// never copy a Map after first use.
type Map[K comparable, V any] struct {
	shards []*shard[K, V]
	seed   maphash.Seed
	mask   uint64
	// snapshots recycles iteration's per-shard copy buffer, so no caller code runs under a lock.
	snapshots sync.Pool
}

// config holds the settings [New] reads from its options.
type config struct {
	concurrency int
	capacity    int
}

// Option configures a [Map] at construction. Pass options to [New].
type Option func(*config)

// WithConcurrency sets the shard count the Map aims for, rounded up to a power of two and clamped to [8, 1024].
func WithConcurrency(n int) Option {
	return func(c *config) { c.concurrency = n }
}

// WithCapacity gives the total number of keys the Map must hold without a
// rehash. The Map divides the hint equally across the shards.
func WithCapacity(n int) Option {
	return func(c *config) { c.capacity = n }
}

// New creates an empty Map. Without options the Map takes the next power of
// two at or above 4*GOMAXPROCS shards, clamped to the range [8, 1024].
func New[K comparable, V any](opts ...Option) *Map[K, V] {
	cfg := config{concurrency: 4 * runtime.GOMAXPROCS(0)}
	for _, opt := range opts {
		opt(&cfg)
	}

	n := shardCount(cfg.concurrency)
	perShard := 0
	if cfg.capacity > 0 {
		perShard = (cfg.capacity + n - 1) / n
	}

	m := &Map[K, V]{
		shards: make([]*shard[K, V], n),
		seed:   maphash.MakeSeed(),
		mask:   uint64(n - 1),
	}
	for i := range m.shards {
		m.shards[i] = &shard[K, V]{m: make(map[K]V, perShard)}
	}
	return m
}

// shardCount clamps want to the bounds and rounds it up to a power of two.
// The count must be a power of two, or the mask cannot select a shard.
func shardCount(want int) int {
	want = min(max(want, minShards), maxShards)
	return 1 << bits.Len(uint(want-1))
}

// mustInit panics when the caller reaches the zero Map.
func (m *Map[K, V]) mustInit() {
	if m.shards == nil {
		panic(errZeroMap)
	}
}

// shard returns the shard that holds key.
func (m *Map[K, V]) shard(key K) *shard[K, V] {
	m.mustInit()
	return m.shards[maphash.Comparable(m.seed, key)&m.mask]
}

// ---------- single-key operations ----------

// Store sets the value for a key. It replaces any previous value.
func (m *Map[K, V]) Store(key K, value V) {
	s := m.shard(key)
	s.mu.Lock()
	s.m[key] = value
	s.mu.Unlock()
}

// Load returns the value stored for key. The second result reports whether
// the key was present.
func (m *Map[K, V]) Load(key K) (V, bool) {
	s := m.shard(key)
	s.mu.RLock()
	v, ok := s.m[key]
	s.mu.RUnlock()
	return v, ok
}

// Contains reports whether the Map holds key.
func (m *Map[K, V]) Contains(key K) bool {
	s := m.shard(key)
	s.mu.RLock()
	_, ok := s.m[key]
	s.mu.RUnlock()
	return ok
}

// TryAdd stores value under key only when the key is absent. It reports
// whether it stored the value.
func (m *Map[K, V]) TryAdd(key K, value V) bool {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[key]; ok {
		return false
	}
	s.m[key] = value
	return true
}

// LoadOrStore returns the existing value for key when the key is present.
// Otherwise it stores value and returns it. The loaded result is true when
// the Map already held the key.
func (m *Map[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	s := m.shard(key)

	s.mu.RLock()
	actual, loaded = s.m[key]
	s.mu.RUnlock()
	if loaded {
		return actual, true
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if actual, loaded = s.m[key]; loaded {
		return actual, true
	}
	s.m[key] = value
	return value, false
}

// LoadOrCompute loads key, or calls fn once under the shard lock to compute
// and store it -- unlike .NET's GetOrAdd, which runs its delegate unlocked.
// fn must not call back into this Map (deadlock) or block (stalls the shard).
func (m *Map[K, V]) LoadOrCompute(key K, fn func(K) V) (actual V, loaded bool) {
	s := m.shard(key)

	s.mu.RLock()
	actual, loaded = s.m[key]
	s.mu.RUnlock()
	if loaded {
		return actual, true
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if actual, loaded = s.m[key]; loaded {
		return actual, true
	}
	actual = fn(key)
	s.m[key] = actual
	return actual, false
}

// AddOrUpdate stores add when key is absent, else stores update(key, old) and
// returns the result. update runs under the shard lock -- [Map.LoadOrCompute]'s rules apply.
func (m *Map[K, V]) AddOrUpdate(key K, add V, update func(key K, old V) V) V {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.m[key]; ok {
		next := update(key, old)
		s.m[key] = next
		return next
	}
	s.m[key] = add
	return add
}

// Compute calls fn once, under the shard lock, with the current value and its
// presence; remove deletes the key, else newValue is stored. [Map.LoadOrCompute]'s rules apply to fn.
func (m *Map[K, V]) Compute(key K, fn func(old V, loaded bool) (newValue V, remove bool)) (V, bool) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	old, loaded := s.m[key]
	next, remove := fn(old, loaded)
	if remove {
		delete(s.m, key)
		var zero V
		return zero, false
	}
	s.m[key] = next
	return next, true
}

// Delete removes key from the Map. It does nothing when the key is absent.
func (m *Map[K, V]) Delete(key K) {
	s := m.shard(key)
	s.mu.Lock()
	delete(s.m, key)
	s.mu.Unlock()
}

// LoadAndDelete removes key and returns the value it held. The second result
// reports whether the key was present.
func (m *Map[K, V]) LoadAndDelete(key K) (V, bool) {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	if ok {
		delete(s.m, key)
	}
	return v, ok
}

// ---------- whole-map operations ----------

// Len locks one shard at a time; exact only while nothing else writes.
func (m *Map[K, V]) Len() int {
	m.mustInit()
	n := 0
	for _, s := range m.shards {
		s.mu.RLock()
		n += len(s.m)
		s.mu.RUnlock()
	}
	return n
}

// IsEmpty reports whether the Map holds no keys. It stops at the first shard
// that holds a key.
func (m *Map[K, V]) IsEmpty() bool {
	m.mustInit()
	for _, s := range m.shards {
		s.mu.RLock()
		n := len(s.m)
		s.mu.RUnlock()
		if n > 0 {
			return false
		}
	}
	return true
}

// Clear removes every key from the Map. Clear locks one shard at a time, so a
// concurrent writer can add a key to a shard Clear already emptied.
func (m *Map[K, V]) Clear() {
	m.mustInit()
	for _, s := range m.shards {
		s.mu.Lock()
		clear(s.m)
		s.mu.Unlock()
	}
}

// takeSnapshot copies one shard into a pooled buffer under the read lock.
func (m *Map[K, V]) takeSnapshot(s *shard[K, V]) *[]pair[K, V] {
	buf, _ := m.snapshots.Get().(*[]pair[K, V])
	if buf == nil {
		buf = new([]pair[K, V])
	}
	*buf = (*buf)[:0]

	s.mu.RLock()
	for k, v := range s.m {
		*buf = append(*buf, pair[K, V]{key: k, value: v})
	}
	s.mu.RUnlock()
	return buf
}

// returnSnapshot puts the buffer back in the pool.
func (m *Map[K, V]) returnSnapshot(buf *[]pair[K, V]) {
	*buf = (*buf)[:0]
	m.snapshots.Put(buf)
}

// All copies one shard under its read lock, releases it, then yields that
// shard's pairs -- a shard-at-a-time view, not one point in time.
func (m *Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		m.mustInit()
		for _, s := range m.shards {
			buf := m.takeSnapshot(s)
			for _, p := range *buf {
				if !yield(p.key, p.value) {
					m.returnSnapshot(buf)
					return
				}
			}
			m.returnSnapshot(buf)
		}
	}
}

// Keys returns an iterator over the keys of the Map. It gives the
// shard-at-a-time view of [Map.All].
func (m *Map[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for k := range m.All() {
			if !yield(k) {
				return
			}
		}
	}
}

// Values returns an iterator over the values of the Map. It gives the
// shard-at-a-time view of [Map.All].
func (m *Map[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, v := range m.All() {
			if !yield(v) {
				return
			}
		}
	}
}

// ToMap returns the contents of the Map as a plain map. It gives the
// shard-at-a-time view of [Map.All].
func (m *Map[K, V]) ToMap() map[K]V {
	out := make(map[K]V, m.Len())
	for k, v := range m.All() {
		out[k] = v
	}
	return out
}

// String returns a human-readable form of the Map. The fmt package sorts the
// keys, so the text is stable for a Map nobody writes to.
func (m *Map[K, V]) String() string {
	return fmt.Sprintf("%v", m.ToMap())
}

// ---------- operations that need a comparable value ----------

// CompareAndSwap stores new for key iff present and equal to old, atomically.
// A function, not a method: a method cannot add V's comparable constraint.
func CompareAndSwap[K comparable, V comparable](m *Map[K, V], key K, old, new V) bool {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.m[key]
	if !ok || cur != old {
		return false
	}
	s.m[key] = new
	return true
}

// CompareAndDelete removes key iff present and equal to old, atomically. A
// function, not a method: a method cannot add V's comparable constraint.
func CompareAndDelete[K comparable, V comparable](m *Map[K, V], key K, old V) bool {
	s := m.shard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.m[key]
	if !ok || cur != old {
		return false
	}
	delete(s.m, key)
	return true
}
