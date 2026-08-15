package set

import (
	"fmt"
	"testing"
)

// Every benchmark here runs twice: once on Set[int], once on the plain
// map[int]struct{} the caller would otherwise write by hand. The pair shares
// its input and its work, so the difference between the two lines is the cost
// of the type, not of the workload.
//
// The hand-rolled side spells out what Set does internally. That is the point:
// a set operation the library exposes as one call is a loop at the call site,
// and both versions of the loop are here to be compared.

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

func BenchmarkContainsHit(b *testing.B) {
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

func BenchmarkContainsMiss(b *testing.B) {
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

func BenchmarkContainsAll(b *testing.B) {
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

func BenchmarkAddNew(b *testing.B) {
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

// BenchmarkAddExisting measures the re-add path. Set.Add reports whether the
// element was new, which costs it a lookup the bare map assignment skips.
func BenchmarkAddExisting(b *testing.B) {
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

func BenchmarkRemove(b *testing.B) {
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

// ---------- set algebra ----------
//
// Each pair holds two 10000-element sets overlapping by half.

func BenchmarkUnion(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			x, y := makeSet(10000, 0), makeSet(10000, 5000)
			b.ResetTimer()
			for range b.N {
				sinkSet = x.Union(y)
			}
		},
		func(b *testing.B) {
			x, y := makeMap(10000, 0), makeMap(10000, 5000)
			b.ResetTimer()
			for range b.N {
				out := make(map[int]struct{}, len(x)+len(y))
				for k := range x {
					out[k] = struct{}{}
				}
				for k := range y {
					out[k] = struct{}{}
				}
				sinkMap = out
			}
		})
}

func BenchmarkIntersection(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			x, y := makeSet(10000, 0), makeSet(10000, 5000)
			b.ResetTimer()
			for range b.N {
				sinkSet = x.Intersection(y)
			}
		},
		func(b *testing.B) {
			x, y := makeMap(10000, 0), makeMap(10000, 5000)
			b.ResetTimer()
			for range b.N {
				out := make(map[int]struct{})
				for k := range x {
					if _, ok := y[k]; ok {
						out[k] = struct{}{}
					}
				}
				sinkMap = out
			}
		})
}

// BenchmarkIntersectionLopsided pits a tiny set against a huge one. Set
// iterates the smaller side; the obvious hand-rolled loop iterates the
// receiver, so the two differ by the size ratio rather than by a constant.
func BenchmarkIntersectionLopsided(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			big, small := makeSet(100000, 0), makeSet(10, 0)
			b.ResetTimer()
			for range b.N {
				sinkSet = big.Intersection(small)
			}
		},
		func(b *testing.B) {
			big, small := makeMap(100000, 0), makeMap(10, 0)
			b.ResetTimer()
			for range b.N {
				out := make(map[int]struct{})
				for k := range big {
					if _, ok := small[k]; ok {
						out[k] = struct{}{}
					}
				}
				sinkMap = out
			}
		})
}

func BenchmarkDifference(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			x, y := makeSet(10000, 0), makeSet(10000, 5000)
			b.ResetTimer()
			for range b.N {
				sinkSet = x.Difference(y)
			}
		},
		func(b *testing.B) {
			x, y := makeMap(10000, 0), makeMap(10000, 5000)
			b.ResetTimer()
			for range b.N {
				out := make(map[int]struct{})
				for k := range x {
					if _, ok := y[k]; !ok {
						out[k] = struct{}{}
					}
				}
				sinkMap = out
			}
		})
}

func BenchmarkSymmetricDifference(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			x, y := makeSet(10000, 0), makeSet(10000, 5000)
			b.ResetTimer()
			for range b.N {
				sinkSet = x.SymmetricDifference(y)
			}
		},
		func(b *testing.B) {
			x, y := makeMap(10000, 0), makeMap(10000, 5000)
			b.ResetTimer()
			for range b.N {
				out := make(map[int]struct{})
				for k := range x {
					if _, ok := y[k]; !ok {
						out[k] = struct{}{}
					}
				}
				for k := range y {
					if _, ok := x[k]; !ok {
						out[k] = struct{}{}
					}
				}
				sinkMap = out
			}
		})
}

// ---------- predicates ----------

func BenchmarkIsSubsetOf(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			small, big := makeSet(1000, 0), makeSet(10000, 0)
			b.ResetTimer()
			for range b.N {
				sinkBool = small.IsSubsetOf(big)
			}
		},
		func(b *testing.B) {
			small, big := makeMap(1000, 0), makeMap(10000, 0)
			b.ResetTimer()
			for range b.N {
				subset := len(small) <= len(big)
				if subset {
					for k := range small {
						if _, ok := big[k]; !ok {
							subset = false
							break
						}
					}
				}
				sinkBool = subset
			}
		})
}

func BenchmarkEqual(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			x, y := makeSet(10000, 0), makeSet(10000, 0)
			b.ResetTimer()
			for range b.N {
				sinkBool = x.Equal(y)
			}
		},
		func(b *testing.B) {
			x, y := makeMap(10000, 0), makeMap(10000, 0)
			b.ResetTimer()
			for range b.N {
				equal := len(x) == len(y)
				if equal {
					for k := range x {
						if _, ok := y[k]; !ok {
							equal = false
							break
						}
					}
				}
				sinkBool = equal
			}
		})
}

// BenchmarkIsDisjoint uses two sets that share nothing, the worst case: no
// early exit is possible, so the whole smaller side is probed.
func BenchmarkIsDisjoint(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			x, y := makeSet(10000, 0), makeSet(10000, 100000)
			b.ResetTimer()
			for range b.N {
				sinkBool = x.IsDisjoint(y)
			}
		},
		func(b *testing.B) {
			x, y := makeMap(10000, 0), makeMap(10000, 100000)
			b.ResetTimer()
			for range b.N {
				disjoint := true
				for k := range x {
					if _, ok := y[k]; ok {
						disjoint = false
						break
					}
				}
				sinkBool = disjoint
			}
		})
}

// ---------- bulk reads ----------

func BenchmarkClone(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			s := makeSet(10000, 0)
			b.ResetTimer()
			for range b.N {
				sinkSet = s.Clone()
			}
		},
		func(b *testing.B) {
			m := makeMap(10000, 0)
			b.ResetTimer()
			for range b.N {
				out := make(map[int]struct{}, len(m))
				for k := range m {
					out[k] = struct{}{}
				}
				sinkMap = out
			}
		})
}

func BenchmarkValues(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			s := makeSet(10000, 0)
			b.ResetTimer()
			for range b.N {
				sinkSlice = s.Values()
			}
		},
		func(b *testing.B) {
			m := makeMap(10000, 0)
			b.ResetTimer()
			for range b.N {
				out := make([]int, 0, len(m))
				for k := range m {
					out = append(out, k)
				}
				sinkSlice = out
			}
		})
}

// BenchmarkIterate walks every element. Set.All returns an iterator function,
// so this measures what that indirection costs against a bare range.
func BenchmarkIterate(b *testing.B) {
	eachSize(b,
		func(b *testing.B, n int) {
			s := makeSet(n, 0)
			b.ResetTimer()
			for range b.N {
				sum := 0
				for k := range s.All() {
					sum += k
				}
				sinkInt = sum
			}
		},
		func(b *testing.B, n int) {
			m := makeMap(n, 0)
			b.ResetTimer()
			for range b.N {
				sum := 0
				for k := range m {
					sum += k
				}
				sinkInt = sum
			}
		})
}

func BenchmarkLen(b *testing.B) {
	pair(b,
		func(b *testing.B) {
			s := makeSet(10000, 0)
			b.ResetTimer()
			for range b.N {
				sinkInt = s.Len()
			}
		},
		func(b *testing.B) {
			m := makeMap(10000, 0)
			b.ResetTimer()
			for range b.N {
				sinkInt = len(m)
			}
		})
}
