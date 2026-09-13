package orderedset

import (
	"fmt"
	"slices"
	"testing"

	"github.com/wow-look-at-my/go-containers/set"
)

// benchSizes are the element counts every size-sensitive benchmark sweeps.
var benchSizes = []int{100, 10000}

// Sinks keep a result alive so the compiler cannot delete the work.
var (
	sinkBool  bool
	sinkSlice []int
	sinkSet   OrderedSet[int]
)

// handrolled is the baseline: a set and a slice kept in step by the caller,
// which has to find an element before it can remove one.
type handrolled struct {
	seen  set.Set[int]
	order []int
}

func newHandrolled(n int) *handrolled {
	return &handrolled{seen: set.New[int](n), order: make([]int, 0, n)}
}

func (h *handrolled) add(v int) bool {
	if !h.seen.Add(v) {
		return false
	}
	h.order = append(h.order, v)
	return true
}

func (h *handrolled) remove(v int) {
	if !h.seen.Contains(v) {
		return
	}
	h.seen.Remove(v)
	at := slices.Index(h.order, v)
	h.order = slices.Delete(h.order, at, at+1)
}

func (h *handrolled) values() []int {
	out := make([]int, len(h.order))
	copy(out, h.order)
	return out
}

// makeSet returns an ordered set holding [off, off+n) in that order.
func makeSet(n, off int) OrderedSet[int] {
	s := New[int](n)
	for i := range n {
		s.Add(i + off)
	}
	return s
}

// makeHandrolled returns the hand-rolled equivalent of makeSet.
func makeHandrolled(n, off int) *handrolled {
	h := newHandrolled(n)
	for i := range n {
		h.add(i + off)
	}
	return h
}

// eachSize runs one pair of implementations at every benchmark size.
func eachSize(b *testing.B, ordered, hand func(b *testing.B, n int)) {
	b.Helper()
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d/orderedset", n), func(b *testing.B) { b.ReportAllocs(); ordered(b, n) })
		b.Run(fmt.Sprintf("n=%d/handrolled", n), func(b *testing.B) { b.ReportAllocs(); hand(b, n) })
	}
}

func BenchmarkCompareAddNew(b *testing.B) {
	eachSize(b,
		func(b *testing.B, n int) {
			for i := range b.N {
				if i%n == 0 {
					sinkSet = New[int](n)
				}
				sinkSet.Add(i % n)
			}
		},
		func(b *testing.B, n int) {
			h := newHandrolled(n)
			for i := range b.N {
				if i%n == 0 {
					h = newHandrolled(n)
				}
				sinkBool = h.add(i % n)
			}
		})
}

func BenchmarkCompareAddExisting(b *testing.B) {
	eachSize(b,
		func(b *testing.B, n int) {
			s := makeSet(n, 0)
			b.ResetTimer()
			for i := range b.N {
				sinkBool = s.Add(i % n)
			}
		},
		func(b *testing.B, n int) {
			h := makeHandrolled(n, 0)
			b.ResetTimer()
			for i := range b.N {
				sinkBool = h.add(i % n)
			}
		})
}

func BenchmarkCompareContainsHit(b *testing.B) {
	eachSize(b,
		func(b *testing.B, n int) {
			s := makeSet(n, 0)
			b.ResetTimer()
			for i := range b.N {
				sinkBool = s.Contains(i % n)
			}
		},
		func(b *testing.B, n int) {
			h := makeHandrolled(n, 0)
			b.ResetTimer()
			for i := range b.N {
				sinkBool = h.seen.Contains(i % n)
			}
		})
}

// The hand-rolled removal is where the index earns its keep: it scans the slice
// to find the element, then shifts the tail down over it.
func BenchmarkCompareRemove(b *testing.B) {
	eachSize(b,
		func(b *testing.B, n int) {
			s := makeSet(n, 0)
			b.ResetTimer()
			for i := range b.N {
				if i%n == 0 {
					s = makeSet(n, 0)
				}
				s.Remove(i % n)
			}
		},
		func(b *testing.B, n int) {
			h := makeHandrolled(n, 0)
			b.ResetTimer()
			for i := range b.N {
				if i%n == 0 {
					h = makeHandrolled(n, 0)
				}
				h.remove(i % n)
			}
		})
}

func BenchmarkCompareValues(b *testing.B) {
	eachSize(b,
		func(b *testing.B, n int) {
			s := makeSet(n, 0)
			b.ResetTimer()
			for range b.N {
				sinkSlice = s.Values()
			}
		},
		func(b *testing.B, n int) {
			h := makeHandrolled(n, 0)
			b.ResetTimer()
			for range b.N {
				sinkSlice = h.values()
			}
		})
}

// Values after heavy removal is the case the dead slots make slower, and the
// case compaction exists to bound.
func BenchmarkCompareValuesAfterRemoval(b *testing.B) {
	eachSize(b,
		func(b *testing.B, n int) {
			s := makeSet(n, 0)
			for i := range n / 2 {
				s.Remove(i * 2)
			}
			b.ResetTimer()
			for range b.N {
				sinkSlice = s.Values()
			}
		},
		func(b *testing.B, n int) {
			h := makeHandrolled(n, 0)
			for i := range n / 2 {
				h.remove(i * 2)
			}
			b.ResetTimer()
			for range b.N {
				sinkSlice = h.values()
			}
		})
}
