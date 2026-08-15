package event

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
)

// Every benchmark here runs the same workload twice: once on Event[T], once on
// the hand-rolled dispatcher below.
//
// The two are not the same thing, and the numbers should be read with that in
// mind. Event buys two guarantees the hand-rolled slice does not have, and the
// gap between the two lines is what they cost:
//
//   - Weak references. Event holds each callback through a weak.Pointer, so a
//     subscriber that goes away does not stay alive through the event, and
//     every Invoke pays a Value() call per callback to learn whether its
//     referent is still there. The slice holds its callbacks strongly and keeps
//     every subscriber alive until someone remembers to unsubscribe.
//   - Re-entrancy. Event copies its callbacks under the read lock and releases
//     the lock before calling any of them, so a callback may subscribe or
//     unsubscribe. The hand-rolled side dispatches while still holding the
//     lock, and a callback that touches the dispatcher there deadlocks. The
//     copy costs no allocation: one subscriber goes to the stack, and more
//     ride a pooled buffer.

// Sinks keep a result alive so the compiler cannot delete the work.
var (
	sinkErr   error
	sinkInt   int
	sinkCount int
)

// handRolled is the dispatcher a caller writes instead: a slice of callbacks
// under a mutex, holding each one strongly.
type handRolled[T any] struct {
	mu        sync.RWMutex
	callbacks []*func(T) error
}

func (h *handRolled[T]) Subscribe(cb *func(T) error) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, existing := range h.callbacks {
		if existing == cb {
			return false
		}
	}
	h.callbacks = append(h.callbacks, cb)
	return true
}

func (h *handRolled[T]) Unsubscribe(cb *func(T) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, existing := range h.callbacks {
		if existing == cb {
			h.callbacks = append(h.callbacks[:i], h.callbacks[i+1:]...)
			return
		}
	}
}

func (h *handRolled[T]) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.callbacks)
}

func (h *handRolled[T]) Invoke(arg T) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var errs []error
	for _, cb := range h.callbacks {
		if err := (*cb)(arg); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// newCallbacks returns n callbacks, retained by the caller so neither
// implementation can collect them mid-benchmark.
func newCallbacks(n int) []func(intArgs) error {
	cbs := make([]func(intArgs) error, n)
	for i := range cbs {
		cbs[i] = func(a intArgs) error {
			sinkInt = a.N
			return nil
		}
	}
	return cbs
}

// subscriberCounts are the callback counts the dispatch benchmarks sweep.
// The middle count is not decoration: dispatch cost per subscriber is what
// the sweep is measuring, and two points cannot show a curve.
var subscriberCounts = []int{1, 10, 100}

// eachCount runs one pair of implementations at every subscriber count.
func eachCount(b *testing.B, event, hand func(b *testing.B, n int)) {
	b.Helper()
	for _, n := range subscriberCounts {
		b.Run(fmt.Sprintf("n=%d/event", n), func(b *testing.B) { b.ReportAllocs(); event(b, n) })
		b.Run(fmt.Sprintf("n=%d/handrolled", n), func(b *testing.B) { b.ReportAllocs(); hand(b, n) })
	}
}

// ---------- dispatch ----------

func BenchmarkCompareInvoke(b *testing.B) {
	eachCount(b,
		func(b *testing.B, n int) {
			var e Event[intArgs]
			cbs := newCallbacks(n)
			for i := range cbs {
				e.Subscribe(&cbs[i])
			}
			arg := intArgs{N: 1}
			b.ResetTimer()
			for range b.N {
				sinkErr = e.Invoke(arg)
			}
			runtime.KeepAlive(cbs)
		},
		func(b *testing.B, n int) {
			var h handRolled[intArgs]
			cbs := newCallbacks(n)
			for i := range cbs {
				h.Subscribe(&cbs[i])
			}
			arg := intArgs{N: 1}
			b.ResetTimer()
			for range b.N {
				sinkErr = h.Invoke(arg)
			}
			runtime.KeepAlive(cbs)
		})
}

// BenchmarkCompareInvokeWithErrors makes every callback fail, so the joined-error
// path is measured rather than the nil-return fast path.
func BenchmarkCompareInvokeWithErrors(b *testing.B) {
	fail := errors.New("boom")
	eachCount(b,
		func(b *testing.B, n int) {
			var e Event[intArgs]
			cbs := make([]func(intArgs) error, n)
			for i := range cbs {
				cbs[i] = func(intArgs) error { return fail }
				e.Subscribe(&cbs[i])
			}
			arg := intArgs{N: 1}
			b.ResetTimer()
			for range b.N {
				sinkErr = e.Invoke(arg)
			}
			runtime.KeepAlive(cbs)
		},
		func(b *testing.B, n int) {
			var h handRolled[intArgs]
			cbs := make([]func(intArgs) error, n)
			for i := range cbs {
				cbs[i] = func(intArgs) error { return fail }
				h.Subscribe(&cbs[i])
			}
			arg := intArgs{N: 1}
			b.ResetTimer()
			for range b.N {
				sinkErr = h.Invoke(arg)
			}
			runtime.KeepAlive(cbs)
		})
}

// BenchmarkCompareConcurrentInvoke dispatches from every core at once. Both
// implementations take a read lock, so this measures contention on it.
func BenchmarkCompareConcurrentInvoke(b *testing.B) {
	eachCount(b,
		func(b *testing.B, n int) {
			var e Event[intArgs]
			cbs := newCallbacks(n)
			for i := range cbs {
				e.Subscribe(&cbs[i])
			}
			arg := intArgs{N: 1}
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					sinkErr = e.Invoke(arg)
				}
			})
			runtime.KeepAlive(cbs)
		},
		func(b *testing.B, n int) {
			var h handRolled[intArgs]
			cbs := newCallbacks(n)
			for i := range cbs {
				h.Subscribe(&cbs[i])
			}
			arg := intArgs{N: 1}
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					sinkErr = h.Invoke(arg)
				}
			})
			runtime.KeepAlive(cbs)
		})
}

// ---------- subscription ----------

// BenchmarkCompareSubscribe fills a fresh dispatcher with 100 callbacks and reports
// the per-callback cost. Event keeps its callbacks in a set, so an insert is a
// hash lookup; the hand-rolled slice scans everything it already holds, which
// is why its cost per subscriber rises with the size of the dispatcher.
func BenchmarkCompareSubscribe(b *testing.B) {
	const n = 100
	pair(b,
		func(b *testing.B) {
			cbs := newCallbacks(n)
			b.ResetTimer()
			for i := 0; i < b.N; i += n {
				var e Event[intArgs]
				for j := range cbs {
					e.Subscribe(&cbs[j])
				}
				sinkCount = e.Len()
			}
			runtime.KeepAlive(cbs)
		},
		func(b *testing.B) {
			cbs := newCallbacks(n)
			b.ResetTimer()
			for i := 0; i < b.N; i += n {
				var h handRolled[intArgs]
				for j := range cbs {
					h.Subscribe(&cbs[j])
				}
				sinkCount = h.Len()
			}
			runtime.KeepAlive(cbs)
		})
}

// BenchmarkCompareSubscribeDuplicate re-adds callbacks that are already registered,
// which is the lookup both implementations do before every insert.
func BenchmarkCompareSubscribeDuplicate(b *testing.B) {
	const n = 100
	pair(b,
		func(b *testing.B) {
			var e Event[intArgs]
			cbs := newCallbacks(n)
			for i := range cbs {
				e.Subscribe(&cbs[i])
			}
			b.ResetTimer()
			for i := range b.N {
				e.Subscribe(&cbs[i%n])
			}
			runtime.KeepAlive(cbs)
		},
		func(b *testing.B) {
			var h handRolled[intArgs]
			cbs := newCallbacks(n)
			for i := range cbs {
				h.Subscribe(&cbs[i])
			}
			b.ResetTimer()
			for i := range b.N {
				h.Subscribe(&cbs[i%n])
			}
			runtime.KeepAlive(cbs)
		})
}

// BenchmarkCompareUnsubscribe removes and re-adds one callback per iteration, so the
// dispatcher keeps its size across the run.
func BenchmarkCompareUnsubscribe(b *testing.B) {
	const n = 100
	pair(b,
		func(b *testing.B) {
			var e Event[intArgs]
			cbs := newCallbacks(n)
			for i := range cbs {
				e.Subscribe(&cbs[i])
			}
			b.ResetTimer()
			for i := range b.N {
				e.Unsubscribe(&cbs[i%n])
				e.Subscribe(&cbs[i%n])
			}
			runtime.KeepAlive(cbs)
		},
		func(b *testing.B) {
			var h handRolled[intArgs]
			cbs := newCallbacks(n)
			for i := range cbs {
				h.Subscribe(&cbs[i])
			}
			b.ResetTimer()
			for i := range b.N {
				h.Unsubscribe(&cbs[i%n])
				h.Subscribe(&cbs[i%n])
			}
			runtime.KeepAlive(cbs)
		})
}

// pair runs one pair of implementations.
func pair(b *testing.B, event, hand func(b *testing.B)) {
	b.Helper()
	b.Run("event", func(b *testing.B) { b.ReportAllocs(); event(b) })
	b.Run("handrolled", func(b *testing.B) { b.ReportAllocs(); hand(b) })
}
