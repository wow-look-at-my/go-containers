package concurrentset

import (
	"hash/maphash"
	"math/bits"
	"runtime"
	"sync"
	"unsafe"

	"github.com/wow-look-at-my/go-containers/set"
)

// directSet is concurrentmap's sharding, hand-coded directly against T with no generic Map layer.
type directSet[T comparable] struct {
	shards []*directShard[T]
	seed   maphash.Seed
	mask   uint64
}

const directShardBytes = 128

// directPadBytes mirrors concurrentmap's padBytes: a set.Set is one map-sized field, same as any pointer-sized map value.
const directPadBytes = directShardBytes - unsafe.Sizeof(sync.RWMutex{}) - unsafe.Sizeof(set.Set[int]{})

type directShard[T comparable] struct {
	mu sync.RWMutex
	s  set.Set[T]
	_  [directPadBytes]byte
}

func newDirectSet[T comparable]() *directSet[T] {
	n := 1 << bits.Len(uint(4*runtime.GOMAXPROCS(0)-1))
	n = min(max(n, 8), 1024)
	ds := &directSet[T]{
		shards: make([]*directShard[T], n),
		seed:   maphash.MakeSeed(),
		mask:   uint64(n - 1),
	}
	for i := range ds.shards {
		ds.shards[i] = &directShard[T]{s: set.New[T]()}
	}
	return ds
}

func (d *directSet[T]) shardFor(v T) *directShard[T] {
	return d.shards[maphash.Comparable(d.seed, v)&d.mask]
}

// Add skips the defer TryAdd uses -- removing every difference isolates the wrapper's own cost.
func (d *directSet[T]) Add(v T) bool {
	s := d.shardFor(v)
	s.mu.Lock()
	added := s.s.Add(v)
	s.mu.Unlock()
	return added
}

func (d *directSet[T]) Contains(v T) bool {
	s := d.shardFor(v)
	s.mu.RLock()
	ok := s.s.Contains(v)
	s.mu.RUnlock()
	return ok
}
