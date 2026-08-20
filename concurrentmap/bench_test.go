package concurrentmap

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// Every benchmark here runs the same workload three times: on Map, on
// sync.Map, and on the RWMutex plus map[int]int a caller writes by hand. The
// three share their key space and their access pattern, so the difference
// between the lines is the cost of the type.
//
// The parallel benchmarks sweep b.SetParallelism, because the whole point of a
// sharded map is how it behaves as the goroutine count grows past the core
// count.

const (
	// mapSize is the number of keys every prepared map holds. It is a power
	// of two, so a key index masks down and needs no divide.
	mapSize = 1 << 12
	keyMask = mapSize - 1

	// hotKeys is the small key space the contended benchmarks share.
	hotKeys = 64
	hotMask = hotKeys - 1
)

// parallelism sweeps the goroutine count as a multiple of GOMAXPROCS.
var parallelism = []int{1, 4, 16}

// Sinks keep a result alive so the compiler cannot delete the work. A parallel
// benchmark folds a goroutine-local total into sinkParallel once, after its
// loop, so the race detector has nothing to report.
var (
	sinkInt      int
	sinkBool     bool
	sinkParallel atomic.Int64
)

// startSeq hands each parallel goroutine a distinct start index. Without it
// every goroutine walks the key space in lockstep and shares one cache line.
var startSeq atomic.Int64

func nextStart() int {
	return int(startSeq.Add(1)) * 977
}

// mutexMap is the guarded map a caller writes when a sharded map is not
// available. One lock covers every key.
type mutexMap struct {
	mu sync.RWMutex
	m  map[int]int
}

func newMutexMap(n int) *mutexMap {
	t := &mutexMap{m: make(map[int]int, n)}
	for i := range n {
		t.m[i] = i
	}
	return t
}

func (t *mutexMap) Load(key int) (int, bool) {
	t.mu.RLock()
	v, ok := t.m[key]
	t.mu.RUnlock()
	return v, ok
}

func (t *mutexMap) Store(key, value int) {
	t.mu.Lock()
	t.m[key] = value
	t.mu.Unlock()
}

func (t *mutexMap) LoadOrStore(key, value int) (int, bool) {
	t.mu.RLock()
	v, ok := t.m[key]
	t.mu.RUnlock()
	if ok {
		return v, true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if v, ok = t.m[key]; ok {
		return v, true
	}
	t.m[key] = value
	return value, false
}

func (t *mutexMap) Len() int {
	t.mu.RLock()
	n := len(t.m)
	t.mu.RUnlock()
	return n
}

func newCMap(n int) *Map[int, int] {
	m := New[int, int](WithCapacity(n))
	for i := range n {
		m.Store(i, i)
	}
	return m
}

func newSyncMap(n int) *sync.Map {
	var m sync.Map
	for i := range n {
		m.Store(i, i)
	}
	return &m
}

// trio runs one workload on all three implementations.
func trio(b *testing.B, cmap, syncmap, mutexmap func(b *testing.B)) {
	b.Helper()
	b.Run("cmap", func(b *testing.B) { b.ReportAllocs(); cmap(b) })
	b.Run("syncmap", func(b *testing.B) { b.ReportAllocs(); syncmap(b) })
	b.Run("mutexmap", func(b *testing.B) { b.ReportAllocs(); mutexmap(b) })
}

// trioParallel runs one workload on all three implementations, at every
// parallelism level.
func trioParallel(b *testing.B, cmap, syncmap, mutexmap func(b *testing.B)) {
	b.Helper()
	for _, p := range parallelism {
		b.Run(fmt.Sprintf("p=%d/cmap", p), func(b *testing.B) { b.ReportAllocs(); b.SetParallelism(p); cmap(b) })
		b.Run(fmt.Sprintf("p=%d/syncmap", p), func(b *testing.B) { b.ReportAllocs(); b.SetParallelism(p); syncmap(b) })
		b.Run(fmt.Sprintf("p=%d/mutexmap", p), func(b *testing.B) { b.ReportAllocs(); b.SetParallelism(p); mutexmap(b) })
	}
}

// ---------- parallel workloads ----------

// BenchmarkCompareLoadParallel is the read-mostly case: every key is present
// and nobody writes.
func BenchmarkCompareLoadParallel(b *testing.B) {
	trioParallel(b,
		func(b *testing.B) {
			m := newCMap(mapSize)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i, sum := nextStart(), 0
				for pb.Next() {
					i++
					if v, ok := m.Load(i & keyMask); ok {
						sum += v
					}
				}
				sinkParallel.Add(int64(sum))
			})
		},
		func(b *testing.B) {
			m := newSyncMap(mapSize)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i, sum := nextStart(), 0
				for pb.Next() {
					i++
					if v, ok := m.Load(i & keyMask); ok {
						sum += v.(int)
					}
				}
				sinkParallel.Add(int64(sum))
			})
		},
		func(b *testing.B) {
			m := newMutexMap(mapSize)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i, sum := nextStart(), 0
				for pb.Next() {
					i++
					if v, ok := m.Load(i & keyMask); ok {
						sum += v
					}
				}
				sinkParallel.Add(int64(sum))
			})
		})
}

// BenchmarkCompareStoreParallel is the write-heavy case: every operation
// overwrites a present key.
func BenchmarkCompareStoreParallel(b *testing.B) {
	trioParallel(b,
		func(b *testing.B) {
			m := newCMap(mapSize)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i := nextStart()
				for pb.Next() {
					i++
					m.Store(i&keyMask, i)
				}
			})
		},
		func(b *testing.B) {
			m := newSyncMap(mapSize)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i := nextStart()
				for pb.Next() {
					i++
					m.Store(i&keyMask, i)
				}
			})
		},
		func(b *testing.B) {
			m := newMutexMap(mapSize)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i := nextStart()
				for pb.Next() {
					i++
					m.Store(i&keyMask, i)
				}
			})
		})
}

// BenchmarkCompareMixedParallel is the 90/10 case: nine reads for each write.
func BenchmarkCompareMixedParallel(b *testing.B) {
	trioParallel(b,
		func(b *testing.B) {
			m := newCMap(mapSize)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i, sum := nextStart(), 0
				for pb.Next() {
					i++
					if i%10 == 0 {
						m.Store(i&keyMask, i)
						continue
					}
					if v, ok := m.Load(i & keyMask); ok {
						sum += v
					}
				}
				sinkParallel.Add(int64(sum))
			})
		},
		func(b *testing.B) {
			m := newSyncMap(mapSize)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i, sum := nextStart(), 0
				for pb.Next() {
					i++
					if i%10 == 0 {
						m.Store(i&keyMask, i)
						continue
					}
					if v, ok := m.Load(i & keyMask); ok {
						sum += v.(int)
					}
				}
				sinkParallel.Add(int64(sum))
			})
		},
		func(b *testing.B) {
			m := newMutexMap(mapSize)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i, sum := nextStart(), 0
				for pb.Next() {
					i++
					if i%10 == 0 {
						m.Store(i&keyMask, i)
						continue
					}
					if v, ok := m.Load(i & keyMask); ok {
						sum += v
					}
				}
				sinkParallel.Add(int64(sum))
			})
		})
}

// BenchmarkCompareLoadOrStoreParallel is the GetOrAdd shape. The key space is
// small, so every goroutine hits the same few keys and the same few shards.
func BenchmarkCompareLoadOrStoreParallel(b *testing.B) {
	trioParallel(b,
		func(b *testing.B) {
			m := New[int, int]()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i, sum := nextStart(), 0
				for pb.Next() {
					i++
					v, _ := m.LoadOrStore(i&hotMask, i)
					sum += v
				}
				sinkParallel.Add(int64(sum))
			})
		},
		func(b *testing.B) {
			var m sync.Map
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i, sum := nextStart(), 0
				for pb.Next() {
					i++
					v, _ := m.LoadOrStore(i&hotMask, i)
					sum += v.(int)
				}
				sinkParallel.Add(int64(sum))
			})
		},
		func(b *testing.B) {
			m := &mutexMap{m: make(map[int]int, hotKeys)}
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i, sum := nextStart(), 0
				for pb.Next() {
					i++
					v, _ := m.LoadOrStore(i&hotMask, i)
					sum += v
				}
				sinkParallel.Add(int64(sum))
			})
		})
}

// ---------- single-goroutine workloads ----------

// BenchmarkCompareLoadSerial removes contention. What is left is the cost of
// the hash, the mask and the indirection through the shard.
func BenchmarkCompareLoadSerial(b *testing.B) {
	trio(b,
		func(b *testing.B) {
			m := newCMap(mapSize)
			b.ResetTimer()
			for i := range b.N {
				_, sinkBool = m.Load(i & keyMask)
			}
		},
		func(b *testing.B) {
			m := newSyncMap(mapSize)
			b.ResetTimer()
			for i := range b.N {
				_, sinkBool = m.Load(i & keyMask)
			}
		},
		func(b *testing.B) {
			m := newMutexMap(mapSize)
			b.ResetTimer()
			for i := range b.N {
				_, sinkBool = m.Load(i & keyMask)
			}
		})
}

func BenchmarkCompareStoreSerial(b *testing.B) {
	trio(b,
		func(b *testing.B) {
			m := newCMap(mapSize)
			b.ResetTimer()
			for i := range b.N {
				m.Store(i&keyMask, i)
			}
		},
		func(b *testing.B) {
			m := newSyncMap(mapSize)
			b.ResetTimer()
			for i := range b.N {
				m.Store(i&keyMask, i)
			}
		},
		func(b *testing.B) {
			m := newMutexMap(mapSize)
			b.ResetTimer()
			for i := range b.N {
				m.Store(i&keyMask, i)
			}
		})
}

// BenchmarkCompareLen counts the keys. sync.Map has no Len, so the caller
// walks the whole map, which is the honest comparison.
func BenchmarkCompareLen(b *testing.B) {
	trio(b,
		func(b *testing.B) {
			m := newCMap(mapSize)
			b.ResetTimer()
			for range b.N {
				sinkInt = m.Len()
			}
		},
		func(b *testing.B) {
			m := newSyncMap(mapSize)
			b.ResetTimer()
			for range b.N {
				n := 0
				m.Range(func(_, _ any) bool {
					n++
					return true
				})
				sinkInt = n
			}
		},
		func(b *testing.B) {
			m := newMutexMap(mapSize)
			b.ResetTimer()
			for range b.N {
				sinkInt = m.Len()
			}
		})
}
