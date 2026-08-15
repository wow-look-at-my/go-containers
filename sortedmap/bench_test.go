package sortedmap

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"sort"
	"testing"
)

// Every benchmark here runs the same workload three ways: on SortedMap, and on
// the two things a caller writes instead.
//
//   - "mapsort" is a plain map[K]V, sorted when the caller needs order. It wins
//     every point lookup and loses whenever order is asked for.
//   - "slice" is a sorted []entry kept in order with a binary search and an
//     insert. It reads fast and pays for every write by moving the tail.
//
// SortedMap is a left-leaning red-black tree, so it sits between them: O(log n)
// for both, and no per-iteration sort. The numbers say where that trade pays.

// benchSizes are the element counts every benchmark sweeps.
var benchSizes = []int{100, 10000}

// Sinks keep a result alive so the compiler cannot delete the work.
var (
	sinkInt   int
	sinkBool  bool
	sinkSlice []int
)

// entry is one key-value pair of the sorted-slice implementation.
type entry struct {
	key   int
	value int
}

// sortedSlice is the hand-rolled ordered map: a slice held in key order.
type sortedSlice []entry

// search returns the index of key, and whether it is present. A miss returns
// the index the entry would be inserted at.
func (s sortedSlice) search(key int) (int, bool) {
	i := sort.Search(len(s), func(i int) bool { return s[i].key >= key })
	return i, i < len(s) && s[i].key == key
}

func (s *sortedSlice) put(key, value int) {
	i, found := s.search(key)
	if found {
		(*s)[i].value = value
		return
	}
	*s = slices.Insert(*s, i, entry{key, value})
}

func (s sortedSlice) get(key int) (int, bool) {
	if i, found := s.search(key); found {
		return s[i].value, true
	}
	return 0, false
}

func (s *sortedSlice) del(key int) bool {
	i, found := s.search(key)
	if !found {
		return false
	}
	*s = slices.Delete(*s, i, i+1)
	return true
}

// keys returns n pseudo-random distinct keys. A benchmark that inserts them in
// order would measure the tree's best case and the slice's worst one.
func keys(n int) []int {
	r := rand.New(rand.NewPCG(1, 2))
	out := r.Perm(n)
	for i := range out {
		out[i] *= 3
	}
	return out
}

func makeTree(ks []int) *SortedMap[int, int] {
	m := New[int, int]()
	for _, k := range ks {
		m.Put(k, k)
	}
	return m
}

func makeMap(ks []int) map[int]int {
	m := make(map[int]int, len(ks))
	for _, k := range ks {
		m[k] = k
	}
	return m
}

func makeSlice(ks []int) sortedSlice {
	var s sortedSlice
	for _, k := range ks {
		s.put(k, k)
	}
	return s
}

// eachSize runs one implementation trio at every benchmark size.
func eachSize(b *testing.B, tree, mapsort, slice func(b *testing.B, ks []int)) {
	b.Helper()
	for _, n := range benchSizes {
		ks := keys(n)
		b.Run(fmt.Sprintf("n=%d/sortedmap", n), func(b *testing.B) { b.ReportAllocs(); tree(b, ks) })
		b.Run(fmt.Sprintf("n=%d/mapsort", n), func(b *testing.B) { b.ReportAllocs(); mapsort(b, ks) })
		b.Run(fmt.Sprintf("n=%d/slice", n), func(b *testing.B) { b.ReportAllocs(); slice(b, ks) })
	}
}

// ---------- writes ----------

// BenchmarkPut builds the whole container from scratch, so the slice's insert
// cost and the tree's rebalancing both land in the measurement.
func BenchmarkPut(b *testing.B) {
	eachSize(b,
		func(b *testing.B, ks []int) {
			b.ResetTimer()
			for i := 0; i < b.N; i += len(ks) {
				m := New[int, int]()
				for _, k := range ks {
					m.Put(k, k)
				}
				sinkInt = m.Len()
			}
		},
		func(b *testing.B, ks []int) {
			b.ResetTimer()
			for i := 0; i < b.N; i += len(ks) {
				m := make(map[int]int, len(ks))
				for _, k := range ks {
					m[k] = k
				}
				sinkInt = len(m)
			}
		},
		func(b *testing.B, ks []int) {
			b.ResetTimer()
			for i := 0; i < b.N; i += len(ks) {
				var s sortedSlice
				for _, k := range ks {
					s.put(k, k)
				}
				sinkInt = len(s)
			}
		})
}

// BenchmarkPutExisting overwrites keys that are already present, so no
// container grows and no element moves.
func BenchmarkPutExisting(b *testing.B) {
	eachSize(b,
		func(b *testing.B, ks []int) {
			m := makeTree(ks)
			b.ResetTimer()
			for i := range b.N {
				m.Put(ks[i%len(ks)], i)
			}
		},
		func(b *testing.B, ks []int) {
			m := makeMap(ks)
			b.ResetTimer()
			for i := range b.N {
				m[ks[i%len(ks)]] = i
			}
		},
		func(b *testing.B, ks []int) {
			s := makeSlice(ks)
			b.ResetTimer()
			for i := range b.N {
				s.put(ks[i%len(ks)], i)
			}
		})
}

// BenchmarkDelete removes and reinserts one key per iteration, so the
// container keeps its size and the timing covers both halves of the churn.
func BenchmarkDelete(b *testing.B) {
	eachSize(b,
		func(b *testing.B, ks []int) {
			m := makeTree(ks)
			b.ResetTimer()
			for i := range b.N {
				k := ks[i%len(ks)]
				m.Delete(k)
				m.Put(k, k)
			}
		},
		func(b *testing.B, ks []int) {
			m := makeMap(ks)
			b.ResetTimer()
			for i := range b.N {
				k := ks[i%len(ks)]
				delete(m, k)
				m[k] = k
			}
		},
		func(b *testing.B, ks []int) {
			s := makeSlice(ks)
			b.ResetTimer()
			for i := range b.N {
				k := ks[i%len(ks)]
				s.del(k)
				s.put(k, k)
			}
		})
}

// ---------- reads ----------

func BenchmarkGetHit(b *testing.B) {
	eachSize(b,
		func(b *testing.B, ks []int) {
			m := makeTree(ks)
			b.ResetTimer()
			for i := range b.N {
				sinkInt, sinkBool = m.Get(ks[i%len(ks)])
			}
		},
		func(b *testing.B, ks []int) {
			m := makeMap(ks)
			b.ResetTimer()
			for i := range b.N {
				sinkInt, sinkBool = m[ks[i%len(ks)]]
			}
		},
		func(b *testing.B, ks []int) {
			s := makeSlice(ks)
			b.ResetTimer()
			for i := range b.N {
				sinkInt, sinkBool = s.get(ks[i%len(ks)])
			}
		})
}

// BenchmarkGetMiss probes keys that are absent: every key here is a multiple
// of three, so the +1 never lands.
func BenchmarkGetMiss(b *testing.B) {
	eachSize(b,
		func(b *testing.B, ks []int) {
			m := makeTree(ks)
			b.ResetTimer()
			for i := range b.N {
				sinkInt, sinkBool = m.Get(ks[i%len(ks)] + 1)
			}
		},
		func(b *testing.B, ks []int) {
			m := makeMap(ks)
			b.ResetTimer()
			for i := range b.N {
				sinkInt, sinkBool = m[ks[i%len(ks)]+1]
			}
		},
		func(b *testing.B, ks []int) {
			s := makeSlice(ks)
			b.ResetTimer()
			for i := range b.N {
				sinkInt, sinkBool = s.get(ks[i%len(ks)] + 1)
			}
		})
}

// ---------- ordered reads ----------
//
// This is what the tree is for. The map has to sort its keys every time the
// caller wants them in order; the tree and the slice are already ordered.

func BenchmarkIterateOrdered(b *testing.B) {
	eachSize(b,
		func(b *testing.B, ks []int) {
			m := makeTree(ks)
			b.ResetTimer()
			for range b.N {
				sum := 0
				for k := range m.All() {
					sum += k
				}
				sinkInt = sum
			}
		},
		func(b *testing.B, ks []int) {
			m := makeMap(ks)
			b.ResetTimer()
			for range b.N {
				order := make([]int, 0, len(m))
				for k := range m {
					order = append(order, k)
				}
				slices.Sort(order)
				sum := 0
				for _, k := range order {
					sum += k
				}
				sinkInt = sum
			}
		},
		func(b *testing.B, ks []int) {
			s := makeSlice(ks)
			b.ResetTimer()
			for range b.N {
				sum := 0
				for _, e := range s {
					sum += e.key
				}
				sinkInt = sum
			}
		})
}

func BenchmarkKeys(b *testing.B) {
	eachSize(b,
		func(b *testing.B, ks []int) {
			m := makeTree(ks)
			b.ResetTimer()
			for range b.N {
				out := make([]int, 0, m.Len())
				for k := range m.Keys() {
					out = append(out, k)
				}
				sinkSlice = out
			}
		},
		func(b *testing.B, ks []int) {
			m := makeMap(ks)
			b.ResetTimer()
			for range b.N {
				out := make([]int, 0, len(m))
				for k := range m {
					out = append(out, k)
				}
				slices.Sort(out)
				sinkSlice = out
			}
		},
		func(b *testing.B, ks []int) {
			s := makeSlice(ks)
			b.ResetTimer()
			for range b.N {
				out := make([]int, 0, len(s))
				for _, e := range s {
					out = append(out, e.key)
				}
				sinkSlice = out
			}
		})
}

// BenchmarkRange walks a 100-key window out of the middle. The map cannot do
// this without ordering everything first, which is the whole gap.
func BenchmarkRange(b *testing.B) {
	eachSize(b,
		func(b *testing.B, ks []int) {
			m := makeTree(ks)
			lo, hi := rangeBounds(ks)
			b.ResetTimer()
			for range b.N {
				sum := 0
				for k := range m.Range(lo, hi) {
					sum += k
				}
				sinkInt = sum
			}
		},
		func(b *testing.B, ks []int) {
			m := makeMap(ks)
			lo, hi := rangeBounds(ks)
			b.ResetTimer()
			for range b.N {
				order := make([]int, 0, len(m))
				for k := range m {
					order = append(order, k)
				}
				slices.Sort(order)
				sum := 0
				for _, k := range order {
					if k >= hi {
						break
					}
					if k >= lo {
						sum += k
					}
				}
				sinkInt = sum
			}
		},
		func(b *testing.B, ks []int) {
			s := makeSlice(ks)
			lo, hi := rangeBounds(ks)
			b.ResetTimer()
			for range b.N {
				i, _ := s.search(lo)
				sum := 0
				for ; i < len(s) && s[i].key < hi; i++ {
					sum += s[i].key
				}
				sinkInt = sum
			}
		})
}

// rangeBounds returns a half-open window over the middle 100 keys.
func rangeBounds(ks []int) (int, int) {
	order := slices.Clone(ks)
	slices.Sort(order)
	lo := len(order) / 2
	hi := min(lo+100, len(order)-1)
	return order[lo], order[hi]
}

// ---------- ordered lookups ----------
//
// Min, Max, Floor and Ceiling have no map equivalent at all: the map has to
// scan every key. The slice reads them off its ends or by binary search.

func BenchmarkMin(b *testing.B) {
	eachSize(b,
		func(b *testing.B, ks []int) {
			m := makeTree(ks)
			b.ResetTimer()
			for range b.N {
				sinkInt, _, sinkBool = m.Min()
			}
		},
		func(b *testing.B, ks []int) {
			m := makeMap(ks)
			b.ResetTimer()
			for range b.N {
				first := true
				best := 0
				for k := range m {
					if first || k < best {
						best, first = k, false
					}
				}
				sinkInt, sinkBool = best, !first
			}
		},
		func(b *testing.B, ks []int) {
			s := makeSlice(ks)
			b.ResetTimer()
			for range b.N {
				sinkInt, sinkBool = s[0].key, len(s) > 0
			}
		})
}

func BenchmarkFloor(b *testing.B) {
	eachSize(b,
		func(b *testing.B, ks []int) {
			m := makeTree(ks)
			b.ResetTimer()
			for i := range b.N {
				sinkInt, _, sinkBool = m.Floor(ks[i%len(ks)] + 1)
			}
		},
		func(b *testing.B, ks []int) {
			m := makeMap(ks)
			b.ResetTimer()
			for i := range b.N {
				target := ks[i%len(ks)] + 1
				best, found := 0, false
				for k := range m {
					if k <= target && (!found || k > best) {
						best, found = k, true
					}
				}
				sinkInt, sinkBool = best, found
			}
		},
		func(b *testing.B, ks []int) {
			s := makeSlice(ks)
			b.ResetTimer()
			for i := range b.N {
				idx, found := s.search(ks[i%len(ks)] + 1)
				if !found {
					idx--
				}
				if idx >= 0 {
					sinkInt, sinkBool = s[idx].key, true
				} else {
					sinkInt, sinkBool = 0, false
				}
			}
		})
}
