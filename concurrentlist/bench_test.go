package concurrentlist

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// Every benchmark here runs the same workload on List and on what a Go caller
// writes instead: a mutex around a slice, and a buffered channel.
//
// The channel is not an equivalent of a List. It cannot grow without a bound,
// it cannot be read without removal, and it has no bulk operation. It is here
// because it is what a Go programmer reaches for, so the comparison is the one
// a reader wants.
//
// Memory is the reason the fill benchmarks build a fresh collection per
// iteration. An append-only benchmark on one shared collection holds every
// element that it ever appends, and the framework raises the iteration count
// until that is gigabytes.

// Sinks keep a result alive so the compiler cannot delete the work.
var (
	sinkInt   int
	sinkBool  bool
	sinkSlice []int
)

// mutexQueue is the first-in-first-out queue that a Go caller writes by hand.
type mutexQueue struct {
	mu    sync.Mutex
	items []int
}

func (q *mutexQueue) Append(v int) {
	q.mu.Lock()
	q.items = append(q.items, v)
	q.mu.Unlock()
}

func (q *mutexQueue) AppendRange(vs []int) {
	q.mu.Lock()
	q.items = append(q.items, vs...)
	q.mu.Unlock()
}

func (q *mutexQueue) TryTake() (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return 0, false
	}
	v := q.items[0]
	q.items = q.items[1:]
	return v, true
}

func (q *mutexQueue) TryTakeRange(buf []int) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := min(len(buf), len(q.items))
	copy(buf[:n], q.items[:n])
	q.items = q.items[n:]
	return n
}

func (q *mutexQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// fillSizes are the element counts the fill benchmarks sweep.
var fillSizes = []int{100, 10000}

// parallelism multiplies GOMAXPROCS in the contention benchmarks.
var parallelism = []int{1, 4, 16}

// eachSize runs one trio of implementations at every fill size.
func eachSize(b *testing.B, list, mutex, channel func(b *testing.B, n int)) {
	b.Helper()
	for _, n := range fillSizes {
		b.Run(fmt.Sprintf("n=%d/list", n), func(b *testing.B) { b.ReportAllocs(); list(b, n) })
		b.Run(fmt.Sprintf("n=%d/mutexslice", n), func(b *testing.B) { b.ReportAllocs(); mutex(b, n) })
		if channel != nil {
			b.Run(fmt.Sprintf("n=%d/channel", n), func(b *testing.B) { b.ReportAllocs(); channel(b, n) })
		}
	}
}

// eachParallelism runs one trio of implementations at every goroutine count.
func eachParallelism(b *testing.B, list, mutex, channel func(b *testing.B)) {
	b.Helper()
	for _, p := range parallelism {
		b.Run(fmt.Sprintf("p=%d/list", p), func(b *testing.B) {
			b.ReportAllocs()
			b.SetParallelism(p)
			list(b)
		})
		b.Run(fmt.Sprintf("p=%d/mutexslice", p), func(b *testing.B) {
			b.ReportAllocs()
			b.SetParallelism(p)
			mutex(b)
		})
		if channel != nil {
			b.Run(fmt.Sprintf("p=%d/channel", p), func(b *testing.B) {
				b.ReportAllocs()
				b.SetParallelism(p)
				channel(b)
			})
		}
	}
}

// ---------- fill and drain, one goroutine ----------

func BenchmarkCompareAppend(b *testing.B) {
	eachSize(b,
		func(b *testing.B, n int) {
			for b.Loop() {
				l := New[int]()
				for i := range n {
					l.Append(i)
				}
				sinkInt = l.Len()
			}
		},
		func(b *testing.B, n int) {
			for b.Loop() {
				q := &mutexQueue{}
				for i := range n {
					q.Append(i)
				}
				sinkInt = q.Len()
			}
		},
		func(b *testing.B, n int) {
			for b.Loop() {
				ch := make(chan int, n)
				for i := range n {
					ch <- i
				}
				sinkInt = len(ch)
			}
		})
}

// The take cost is this benchmark minus BenchmarkCompareAppend at the same
// size. Measuring the drain alone needs a full collection per iteration, and
// the timer games that build one distort the small sizes.
func BenchmarkCompareFillAndDrain(b *testing.B) {
	eachSize(b,
		func(b *testing.B, n int) {
			for b.Loop() {
				l := New[int]()
				for i := range n {
					l.Append(i)
				}
				for range n {
					sinkInt, sinkBool = l.TryTake()
				}
			}
		},
		func(b *testing.B, n int) {
			for b.Loop() {
				q := &mutexQueue{}
				for i := range n {
					q.Append(i)
				}
				for range n {
					sinkInt, sinkBool = q.TryTake()
				}
			}
		},
		func(b *testing.B, n int) {
			for b.Loop() {
				ch := make(chan int, n)
				for i := range n {
					ch <- i
				}
				for range n {
					sinkInt = <-ch
				}
			}
		})
}

// ---------- contention ----------

// A round trip is one append and one take. It keeps the collection at a steady
// size, so the number measures the contention, not the memory that grows under
// it.
func BenchmarkCompareRoundTrip(b *testing.B) {
	const prefill = 1024

	eachParallelism(b,
		func(b *testing.B) {
			l := New[int]()
			for i := range prefill {
				l.Append(i)
			}
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					l.Append(1)
					sinkInt, sinkBool = l.TryTake()
				}
			})
		},
		func(b *testing.B) {
			q := &mutexQueue{}
			for i := range prefill {
				q.Append(i)
			}
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					q.Append(1)
					sinkInt, sinkBool = q.TryTake()
				}
			})
		},
		func(b *testing.B) {
			ch := make(chan int, prefill*2)
			for i := range prefill {
				ch <- i
			}
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					ch <- 1
					sinkInt = <-ch
				}
			})
		})
}

// Producers only. The takes run on the same collection, so the appends contend
// with each other and with the drain.
func BenchmarkCompareAppendContended(b *testing.B) {
	eachParallelism(b,
		func(b *testing.B) {
			l := New[int]()
			done := drain(func() bool { _, ok := l.TryTake(); return ok })
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					l.Append(1)
				}
			})
			b.StopTimer()
			close(done)
		},
		func(b *testing.B) {
			q := &mutexQueue{}
			done := drain(func() bool { _, ok := q.TryTake(); return ok })
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					q.Append(1)
				}
			})
			b.StopTimer()
			close(done)
		},
		nil)
}

// drain starts one goroutine that removes elements until the caller closes the
// returned channel. It keeps an append-only benchmark from holding every
// element it appends.
func drain(take func() bool) chan struct{} {
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				take()
			}
		}
	}()
	return done
}

// ---------- bulk ----------

// One atomic add reserves a whole run of slots, so a bulk append should cost
// far less than the same number of single appends.
func BenchmarkCompareAppendRange(b *testing.B) {
	const batch = 64

	values := make([]int, batch)
	for i := range values {
		values[i] = i
	}

	b.Run("list/bulk", func(b *testing.B) {
		b.ReportAllocs()
		l := New[int]()
		buf := make([]int, batch)
		for b.Loop() {
			l.AppendRange(values...)
			sinkInt = l.TryTakeRange(buf)
		}
	})
	b.Run("list/oneByOne", func(b *testing.B) {
		b.ReportAllocs()
		l := New[int]()
		for b.Loop() {
			for _, v := range values {
				l.Append(v)
			}
			for range values {
				sinkInt, sinkBool = l.TryTake()
			}
		}
	})
	b.Run("mutexslice/bulk", func(b *testing.B) {
		b.ReportAllocs()
		q := &mutexQueue{}
		buf := make([]int, batch)
		for b.Loop() {
			q.AppendRange(values)
			sinkInt = q.TryTakeRange(buf)
		}
	})
	b.Run("mutexslice/oneByOne", func(b *testing.B) {
		b.ReportAllocs()
		q := &mutexQueue{}
		for b.Loop() {
			for _, v := range values {
				q.Append(v)
			}
			for range values {
				sinkInt, sinkBool = q.TryTake()
			}
		}
	})
	b.Run("channel/oneByOne", func(b *testing.B) {
		b.ReportAllocs()
		ch := make(chan int, batch)
		for b.Loop() {
			for _, v := range values {
				ch <- v
			}
			for range values {
				sinkInt = <-ch
			}
		}
	})
}

// ---------- reads ----------

func BenchmarkCompareLen(b *testing.B) {
	l := New[int]()
	q := &mutexQueue{}
	for i := range 1000 {
		l.Append(i)
		q.Append(i)
	}

	b.Run("list", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkInt = l.Len()
		}
	})
	b.Run("mutexslice", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkInt = q.Len()
		}
	})
}

func BenchmarkCompareValues(b *testing.B) {
	l := New[int]()
	q := &mutexQueue{}
	for i := range 10000 {
		l.Append(i)
		q.Append(i)
	}

	b.Run("list", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkSlice = l.Values()
		}
	})
	b.Run("mutexslice", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			q.mu.Lock()
			sinkSlice = append([]int(nil), q.items...)
			q.mu.Unlock()
		}
	})
}

// ---------- blocking ----------

// The blocking list against the Go answer to the same problem: a buffered
// channel with one producer and one consumer.
func BenchmarkCompareBlockingProducerConsumer(b *testing.B) {
	const capacity = 256

	b.Run("blockinglist", func(b *testing.B) {
		b.ReportAllocs()
		ctx := context.Background()
		bl := NewBlocking[int](WithCapacity(capacity))
		var wg sync.WaitGroup
		wg.Go(func() {
			for v := range bl.Consume(ctx) {
				sinkInt = v
			}
		})
		for b.Loop() {
			if err := bl.Append(ctx, 1); err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
		bl.CompleteAdding()
		wg.Wait()
	})
	b.Run("channel", func(b *testing.B) {
		b.ReportAllocs()
		ch := make(chan int, capacity)
		var wg sync.WaitGroup
		wg.Go(func() {
			for v := range ch {
				sinkInt = v
			}
		})
		for b.Loop() {
			ch <- 1
		}
		b.StopTimer()
		close(ch)
		wg.Wait()
	})
}
