package concurrentset

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/wow-look-at-my/go-containers/set"
)

// BenchmarkCompare* runs Set against a sync.Mutex guarding a set.Set.
var parallelism = []int{1, 4, 16}

var sinkBool bool

// mutexSet is a sync.Mutex guarding a set.Set[int] -- the baseline a Go
// caller reaches for without this package.
type mutexSet struct {
	mu sync.Mutex
	s  set.Set[int]
}

func newMutexSet() *mutexSet { return &mutexSet{s: set.New[int]()} }

func (m *mutexSet) Add(v int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.s.Add(v)
}

func (m *mutexSet) Contains(v int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.s.Contains(v)
}

// startSeq keeps parallel goroutines off the same cache line.
var startSeq atomic.Int64

func nextStart() int {
	return int(startSeq.Add(1)) * 977
}

const (
	hotKeys = 64
	hotMask = hotKeys - 1
)

func BenchmarkCompareAddContended(b *testing.B) {
	for _, p := range parallelism {
		b.Run(benchName(p, "Set"), func(b *testing.B) {
			s := New[int]()
			b.SetParallelism(p)
			b.RunParallel(func(pb *testing.PB) {
				i := nextStart()
				for pb.Next() {
					sinkBool = s.Add(i & hotMask)
					i++
				}
			})
		})
		b.Run(benchName(p, "MutexSet"), func(b *testing.B) {
			m := newMutexSet()
			b.SetParallelism(p)
			b.RunParallel(func(pb *testing.PB) {
				i := nextStart()
				for pb.Next() {
					sinkBool = m.Add(i & hotMask)
					i++
				}
			})
		})
		b.Run(benchName(p, "DirectSet"), func(b *testing.B) {
			d := newDirectSet[int]()
			b.SetParallelism(p)
			b.RunParallel(func(pb *testing.PB) {
				i := nextStart()
				for pb.Next() {
					sinkBool = d.Add(i & hotMask)
					i++
				}
			})
		})
	}
}

func BenchmarkCompareContainsContended(b *testing.B) {
	for _, p := range parallelism {
		b.Run(benchName(p, "Set"), func(b *testing.B) {
			s := New[int]()
			for k := 0; k < hotKeys; k++ {
				s.Add(k)
			}
			b.SetParallelism(p)
			b.RunParallel(func(pb *testing.PB) {
				i := nextStart()
				for pb.Next() {
					sinkBool = s.Contains(i & hotMask)
					i++
				}
			})
		})
		b.Run(benchName(p, "MutexSet"), func(b *testing.B) {
			m := newMutexSet()
			for k := 0; k < hotKeys; k++ {
				m.Add(k)
			}
			b.SetParallelism(p)
			b.RunParallel(func(pb *testing.PB) {
				i := nextStart()
				for pb.Next() {
					sinkBool = m.Contains(i & hotMask)
					i++
				}
			})
		})
		b.Run(benchName(p, "DirectSet"), func(b *testing.B) {
			d := newDirectSet[int]()
			for k := 0; k < hotKeys; k++ {
				d.Add(k)
			}
			b.SetParallelism(p)
			b.RunParallel(func(pb *testing.PB) {
				i := nextStart()
				for pb.Next() {
					sinkBool = d.Contains(i & hotMask)
					i++
				}
			})
		})
	}
}

func benchName(p int, impl string) string {
	return "p=" + strconv.Itoa(p) + "/" + impl
}
