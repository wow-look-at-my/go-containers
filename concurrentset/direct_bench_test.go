package concurrentset

import (
	"hash/maphash"
	"math/bits"
	"runtime"
	"sync"
	"unsafe"
)

// directSet is a hand-specialized concurrent set: the same sharding strategy
// as concurrentmap (maphash.Comparable picks a padded, independently locked
// shard), coded directly against T and struct{} instead of routed through
// concurrentmap.Map[T, struct{}]'s generic API. It exists only to benchmark
// against Set, to check whether the wrapper's extra call layer costs
// anything a caller who hand-rolled a set from scratch would not also pay.
type directSet[T comparable] struct {
	shards []*directShard[T]
	seed   maphash.Seed
	mask   uint64
}

const directShardBytes = 128

// directPadBytes mirrors concurrentmap's padBytes: struct{} costs the same as any pointer-sized map value.
const directPadBytes = directShardBytes - unsafe.Sizeof(sync.RWMutex{}) - unsafe.Sizeof(map[int]struct{}(nil))

type directShard[T comparable] struct {
	mu sync.RWMutex
	m  map[T]struct{}
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
		ds.shards[i] = &directShard[T]{m: make(map[T]struct{})}
	}
	return ds
}

func (d *directSet[T]) shardFor(v T) *directShard[T] {
	return d.shards[maphash.Comparable(d.seed, v)&d.mask]
}

// Add has no defer, unlike concurrentmap.Map.TryAdd -- removing every difference lets the benchmark isolate the wrapper's own cost.
func (d *directSet[T]) Add(v T) bool {
	s := d.shardFor(v)
	s.mu.Lock()
	if _, ok := s.m[v]; ok {
		s.mu.Unlock()
		return false
	}
	s.m[v] = struct{}{}
	s.mu.Unlock()
	return true
}

func (d *directSet[T]) Contains(v T) bool {
	s := d.shardFor(v)
	s.mu.RLock()
	_, ok := s.m[v]
	s.mu.RUnlock()
	return ok
}
