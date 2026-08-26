package set

import (
	"fmt"
	"testing"
)

// Every benchmark runs Set[int] against a hand-rolled map[int]struct{}.

// benchSizes are the element counts every size-sensitive benchmark sweeps.
var benchSizes = []int{100, 10000}

// Sinks keep a result alive so the compiler cannot delete the work.
var (
	sinkBool  bool
	sinkInt   int
	sinkSlice []int
	sinkSet   Set[int]
	sinkMap   map[int]struct{}
)

// makeSet returns a set holding [off, off+n).
func makeSet(n, off int) Set[int] {
	s := New[int](n)
	for i := range n {
		s.Add(i + off)
	}
	return s
}

// makeMap returns the hand-rolled equivalent of makeSet.
func makeMap(n, off int) map[int]struct{} {
	m := make(map[int]struct{}, n)
	for i := range n {
		m[i+off] = struct{}{}
	}
	return m
}

// eachSize runs one pair of implementations at every benchmark size.
func eachSize(b *testing.B, set, hand func(b *testing.B, n int)) {
	b.Helper()
	for _, n := range benchSizes {
		b.Run(fmt.Sprintf("n=%d/set", n), func(b *testing.B) { b.ReportAllocs(); set(b, n) })
		b.Run(fmt.Sprintf("n=%d/handrolled", n), func(b *testing.B) { b.ReportAllocs(); hand(b, n) })
	}
}

// pair runs one pair of implementations at a single size.
func pair(b *testing.B, set, hand func(b *testing.B)) {
	b.Helper()
	b.Run("set", func(b *testing.B) { b.ReportAllocs(); set(b) })
	b.Run("handrolled", func(b *testing.B) { b.ReportAllocs(); hand(b) })
}

// ---------- membership ----------

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
			m := makeMap(n, 0)
			b.ResetTimer()
			for i := range b.N {
				_, sinkBool = m[i%n]
			}
		})
}

func BenchmarkCompareContainsMiss(b *testing.B) {
	eachSize(b,
		func(b *testing.B, n int) {
			s := makeSet(n, 0)
			b.ResetTimer()
			for i := range b.N {
				sinkBool = s.Contains(n + i%n)
			}
		},
		func(b *testing.B, n int) {
			m := makeMap(n, 0)
			b.ResetTimer()
			for i := range b.N {
				_, sinkBool = m[n+i%n]
			}
		})
}

func BenchmarkCompareContainsAll(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			s := makeSet(10000, 0)
			want := []int{1, 2, 3, 4, 5, 6, 7, 8}
			b.ResetTimer()
			for range b.N {
				sinkBool = s.ContainsAll(want...)
			}
		},
		func(b *testing.B) {
			m := makeMap(10000, 0)
			want := []int{1, 2, 3, 4, 5, 6, 7, 8}
			b.ResetTimer()
			for range b.N {
				all := true
				for _, k := range want {
					if _, ok := m[k]; !ok {
						all = false
						break
					}
				}
				sinkBool = all
			}
		})
}

// ---------- mutation ----------

func BenchmarkCompareAddNew(b *testing.B) {
	eachSize(b,
		func(b *testing.B, n int) {
			b.ResetTimer()
			for i := 0; i < b.N; i += n {
				s := New[int](n)
				for j := range n {
					s.Add(j)
				}
				sinkSet = s
			}
		},
		func(b *testing.B, n int) {
			b.ResetTimer()
			for i := 0; i < b.N; i += n {
				m := make(map[int]struct{}, n)
				for j := range n {
					m[j] = struct{}{}
				}
				sinkMap = m
			}
		})
}

// BenchmarkCompareAddExisting measures the re-add path. Set.Add reports whether the
// element was new, which costs it a lookup the bare map assignment skips.
func BenchmarkCompareAddExisting(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			s := makeSet(10000, 0)
			b.ResetTimer()
			for i := range b.N {
				sinkBool = s.Add(i % 10000)
			}
		},
		func(b *testing.B) {
			m := makeMap(10000, 0)
			b.ResetTimer()
			for i := range b.N {
				k := i % 10000
				_, existed := m[k]
				m[k] = struct{}{}
				sinkBool = !existed
			}
		})
}

func BenchmarkCompareRemove(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			s := makeSet(10000, 0)
			b.ResetTimer()
			for i := range b.N {
				s.Remove(i % 10000)
			}
		},
		func(b *testing.B) {
			m := makeMap(10000, 0)
			b.ResetTimer()
			for i := range b.N {
				delete(m, i%10000)
			}
		})
}
