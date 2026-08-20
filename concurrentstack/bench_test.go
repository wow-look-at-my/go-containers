package concurrentstack

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// Every benchmark here runs one workload on Stack[int] and on what a caller
// writes instead: a sync.Mutex around a slice. The pair shares its workload, so
// the difference between the two lines is the cost of the type.
//
// A benchmark that both adds and takes runs a buffered channel too. A channel
// is first-in-first-out, so it is not a stack and it never appears in an
// order-sensitive benchmark. It is here for throughput comparison only.

// benchParallelism sweeps the goroutine count. Each value multiplies GOMAXPROCS.
var benchParallelism = []int{1, 4, 16}

const (
	// benchBatch is the size of one PushRange or TryPopRange.
	benchBatch = 64
	// benchDrain bounds a push-only benchmark. One drain per benchDrain pushes
	// keeps memory flat and costs under a thousandth of an operation.
	benchDrain = 4096
	// benchFill is the prefill of a pop-only benchmark.
	benchFill = 1 << 16
	// benchChanCap holds every value in flight. Each goroutine sends one value
	// before it receives one, so the send never blocks.
	benchChanCap = 1 << 16
)

// Sinks keep a result alive so the compiler cannot delete the work.
var (
	sinkInt      int
	sinkBool     bool
	parallelSink atomic.Int64
)

// mutexStack is the stack a caller writes without this package: a mutex around
// a slice, with an append to push and a truncation to pop.
type mutexStack[T any] struct {
	mu    sync.Mutex
	items []T
}

func (s *mutexStack[T]) Push(value T) {
	s.mu.Lock()
	s.items = append(s.items, value)
	s.mu.Unlock()
}

func (s *mutexStack[T]) PushRange(values ...T) {
	s.mu.Lock()
	s.items = append(s.items, values...)
	s.mu.Unlock()
}

func (s *mutexStack[T]) TryPop() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	last := len(s.items) - 1
	v := s.items[last]
	s.items = s.items[:last]
	return v, true
}

func (s *mutexStack[T]) TryPopRange(buf []T) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := min(len(buf), len(s.items))
	for i := range n {
		buf[i] = s.items[len(s.items)-1-i]
	}
	s.items = s.items[:len(s.items)-n]
	return n
}

func (s *mutexStack[T]) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

// impl is one implementation of the workload under measurement.
type impl struct {
	name string
	run  func(b *testing.B)
}

// eachParallelism runs every implementation at every goroutine count.
func eachParallelism(b *testing.B, impls ...impl) {
	b.Helper()
	for _, p := range benchParallelism {
		for _, im := range impls {
			b.Run(fmt.Sprintf("p=%d/%s", p, im.name), func(b *testing.B) {
				b.ReportAllocs()
				b.SetParallelism(p)
				im.run(b)
			})
		}
	}
}

// each runs every implementation once, on one goroutine.
func each(b *testing.B, impls ...impl) {
	b.Helper()
	for _, im := range impls {
		b.Run(im.name, func(b *testing.B) {
			b.ReportAllocs()
			im.run(b)
		})
	}
}

// ---------- parallel ----------

func BenchmarkCompareParallelPush(b *testing.B) {
	eachParallelism(b,
		impl{"stack", func(b *testing.B) {
			s := New[int]()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				buf := make([]int, benchDrain)
				i := 0
				for pb.Next() {
					s.Push(i)
					i++
					if i%benchDrain == 0 {
						s.TryPopRange(buf)
					}
				}
			})
		}},
		impl{"mutexslice", func(b *testing.B) {
			s := &mutexStack[int]{}
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				buf := make([]int, benchDrain)
				i := 0
				for pb.Next() {
					s.Push(i)
					i++
					if i%benchDrain == 0 {
						s.TryPopRange(buf)
					}
				}
			})
		}})
}

func BenchmarkCompareParallelPop(b *testing.B) {
	// A goroutine that finds the stack empty refills it. The refill is one
	// PushRange per benchBatch pops, so the measurement stays a pop.
	refill := make([]int, benchBatch)

	eachParallelism(b,
		impl{"stack", func(b *testing.B) {
			s := New[int]()
			for i := range benchFill {
				s.Push(i)
			}
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				local := 0
				for pb.Next() {
					v, ok := s.TryPop()
					if !ok {
						s.PushRange(refill...)
						v, _ = s.TryPop()
					}
					local += v
				}
				parallelSink.Add(int64(local))
			})
		}},
		impl{"mutexslice", func(b *testing.B) {
			s := &mutexStack[int]{}
			for i := range benchFill {
				s.Push(i)
			}
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				local := 0
				for pb.Next() {
					v, ok := s.TryPop()
					if !ok {
						s.PushRange(refill...)
						v, _ = s.TryPop()
					}
					local += v
				}
				parallelSink.Add(int64(local))
			})
		}})
}

func BenchmarkCompareParallelPushPop(b *testing.B) {
	eachParallelism(b,
		impl{"stack", func(b *testing.B) {
			s := New[int]()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				local := 0
				for pb.Next() {
					s.Push(1)
					v, _ := s.TryPop()
					local += v
				}
				parallelSink.Add(int64(local))
			})
		}},
		impl{"mutexslice", func(b *testing.B) {
			s := &mutexStack[int]{}
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				local := 0
				for pb.Next() {
					s.Push(1)
					v, _ := s.TryPop()
					local += v
				}
				parallelSink.Add(int64(local))
			})
		}},
		impl{"channel", func(b *testing.B) {
			ch := make(chan int, benchChanCap)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				local := 0
				for pb.Next() {
					ch <- 1
					local += <-ch
				}
				parallelSink.Add(int64(local))
			})
		}})
}

func BenchmarkCompareParallelBulk(b *testing.B) {
	// One iteration moves benchBatch values in and back out again, so the
	// stack stays small whatever b.N is.
	values := make([]int, benchBatch)

	eachParallelism(b,
		impl{"stack", func(b *testing.B) {
			s := New[int]()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				buf := make([]int, benchBatch)
				local := 0
				for pb.Next() {
					s.PushRange(values...)
					local += s.TryPopRange(buf)
				}
				parallelSink.Add(int64(local))
			})
		}},
		impl{"mutexslice", func(b *testing.B) {
			s := &mutexStack[int]{}
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				buf := make([]int, benchBatch)
				local := 0
				for pb.Next() {
					s.PushRange(values...)
					local += s.TryPopRange(buf)
				}
				parallelSink.Add(int64(local))
			})
		}})
}

func BenchmarkCompareParallelLen(b *testing.B) {
	eachParallelism(b,
		impl{"stack", func(b *testing.B) {
			s := New[int]()
			s.PushRange(make([]int, benchBatch)...)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				local := 0
				for pb.Next() {
					local += s.Len()
				}
				parallelSink.Add(int64(local))
			})
		}},
		impl{"mutexslice", func(b *testing.B) {
			s := &mutexStack[int]{}
			s.PushRange(make([]int, benchBatch)...)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				local := 0
				for pb.Next() {
					local += s.Len()
				}
				parallelSink.Add(int64(local))
			})
		}})
}

// ---------- one goroutine ----------

func BenchmarkComparePush(b *testing.B) {
	each(b,
		impl{"stack", func(b *testing.B) {
			s := New[int]()
			buf := make([]int, benchDrain)
			b.ResetTimer()
			for i := range b.N {
				s.Push(i)
				if i%benchDrain == benchDrain-1 {
					s.TryPopRange(buf)
				}
			}
		}},
		impl{"mutexslice", func(b *testing.B) {
			s := &mutexStack[int]{}
			buf := make([]int, benchDrain)
			b.ResetTimer()
			for i := range b.N {
				s.Push(i)
				if i%benchDrain == benchDrain-1 {
					s.TryPopRange(buf)
				}
			}
		}})
}

func BenchmarkComparePop(b *testing.B) {
	refill := make([]int, benchBatch)

	each(b,
		impl{"stack", func(b *testing.B) {
			s := New[int]()
			for i := range benchFill {
				s.Push(i)
			}
			b.ResetTimer()
			for range b.N {
				v, ok := s.TryPop()
				if !ok {
					s.PushRange(refill...)
					v, _ = s.TryPop()
				}
				sinkInt = v
			}
		}},
		impl{"mutexslice", func(b *testing.B) {
			s := &mutexStack[int]{}
			for i := range benchFill {
				s.Push(i)
			}
			b.ResetTimer()
			for range b.N {
				v, ok := s.TryPop()
				if !ok {
					s.PushRange(refill...)
					v, _ = s.TryPop()
				}
				sinkInt = v
			}
		}})
}

func BenchmarkComparePushPop(b *testing.B) {
	each(b,
		impl{"stack", func(b *testing.B) {
			s := New[int]()
			b.ResetTimer()
			for i := range b.N {
				s.Push(i)
				sinkInt, sinkBool = s.TryPop()
			}
		}},
		impl{"mutexslice", func(b *testing.B) {
			s := &mutexStack[int]{}
			b.ResetTimer()
			for i := range b.N {
				s.Push(i)
				sinkInt, sinkBool = s.TryPop()
			}
		}},
		impl{"channel", func(b *testing.B) {
			ch := make(chan int, benchChanCap)
			b.ResetTimer()
			for i := range b.N {
				ch <- i
				sinkInt = <-ch
			}
		}})
}

func BenchmarkCompareBulk(b *testing.B) {
	values := make([]int, benchBatch)

	each(b,
		impl{"stack", func(b *testing.B) {
			s := New[int]()
			buf := make([]int, benchBatch)
			b.ResetTimer()
			for range b.N {
				s.PushRange(values...)
				sinkInt = s.TryPopRange(buf)
			}
		}},
		impl{"mutexslice", func(b *testing.B) {
			s := &mutexStack[int]{}
			buf := make([]int, benchBatch)
			b.ResetTimer()
			for range b.N {
				s.PushRange(values...)
				sinkInt = s.TryPopRange(buf)
			}
		}})
}

func BenchmarkCompareLen(b *testing.B) {
	each(b,
		impl{"stack", func(b *testing.B) {
			s := New[int]()
			s.PushRange(make([]int, benchBatch)...)
			b.ResetTimer()
			for range b.N {
				sinkInt = s.Len()
			}
		}},
		impl{"mutexslice", func(b *testing.B) {
			s := &mutexStack[int]{}
			s.PushRange(make([]int, benchBatch)...)
			b.ResetTimer()
			for range b.N {
				sinkInt = s.Len()
			}
		}})
}
