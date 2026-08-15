package set

import "testing"

// The algebra half of the comparison suite. Its harness -- makeSet, makeMap,
// pair, and the sinks -- lives in bench_test.go.

// ---------- set algebra ----------
//
// Each pair holds two 10000-element sets overlapping by half.

func BenchmarkCompareUnion(b *testing.B) {
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

func BenchmarkCompareIntersection(b *testing.B) {
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

// BenchmarkCompareIntersectionLopsided pits a tiny set against a huge one. Set
// iterates the smaller side; the obvious hand-rolled loop iterates the
// receiver, so the two differ by the size ratio rather than by a constant.
func BenchmarkCompareIntersectionLopsided(b *testing.B) {
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

func BenchmarkCompareDifference(b *testing.B) {
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

func BenchmarkCompareSymmetricDifference(b *testing.B) {
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

func BenchmarkCompareIsSubsetOf(b *testing.B) {
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

func BenchmarkCompareEqual(b *testing.B) {
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

// BenchmarkCompareIsDisjoint uses two sets that share nothing, the worst case: no
// early exit is possible, so the whole smaller side is probed.
func BenchmarkCompareIsDisjoint(b *testing.B) {
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

func BenchmarkCompareClone(b *testing.B) {
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

func BenchmarkCompareValues(b *testing.B) {
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

// BenchmarkCompareIterate walks every element. Set.All returns an iterator function,
// so this measures what that indirection costs against a bare range.
func BenchmarkCompareIterate(b *testing.B) {
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

func BenchmarkCompareLen(b *testing.B) {
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
