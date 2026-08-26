// Package concurrentqueue provides Queue, a lock-free first-in-first-out
// collection for concurrent use, under .NET's ConcurrentQueue vocabulary
// (Enqueue/TryDequeue) rather than concurrentlist's neutral one (Append/Take).
package concurrentqueue

import (
	"iter"

	"github.com/wow-look-at-my/go-containers/concurrentlist"
)

// Queue is a first-in-first-out collection safe for concurrent use: it is
// concurrentlist.List under queue vocabulary, not a second implementation.
// The zero value is ready to use.
type Queue[T any] struct {
	list concurrentlist.List[T]
}

// New creates an empty queue. The zero Queue is equally usable; New exists
// for callers that want a pointer in one expression.
func New[T any]() *Queue[T] {
	return &Queue[T]{}
}

// Enqueue adds value to the back of the queue. It never blocks.
func (q *Queue[T]) Enqueue(value T) {
	q.list.Append(value)
}

// EnqueueRange adds every value to the back of the queue, in the given
// order, with one atomic reservation for the whole run.
func (q *Queue[T]) EnqueueRange(values ...T) {
	q.list.AppendRange(values...)
}

// TryDequeue removes and returns the oldest value. It reports false when the
// queue is empty.
func (q *Queue[T]) TryDequeue() (T, bool) {
	return q.list.TryTake()
}

// TryDequeueRange removes up to len(buf) values into buf, oldest first, and
// returns how many it wrote.
func (q *Queue[T]) TryDequeueRange(buf []T) int {
	return q.list.TryTakeRange(buf)
}

// TryPeek returns the oldest value and leaves it in the queue, or false when
// the queue is empty. Another goroutine can dequeue it before the caller reads it.
func (q *Queue[T]) TryPeek() (T, bool) {
	return q.list.TryPeek()
}

// Len returns the number of values in the queue. It is a snapshot.
func (q *Queue[T]) Len() int {
	return q.list.Len()
}

// IsEmpty reports whether the queue holds no values. It is a snapshot, with
// the same limits as [Queue.Len].
func (q *Queue[T]) IsEmpty() bool {
	return q.list.IsEmpty()
}

// Clear removes values from the queue until it looks empty. Values that
// another goroutine enqueues during the call can survive it.
func (q *Queue[T]) Clear() {
	q.list.Clear()
}

// All iterates the values oldest first, taking none. It is a best-effort
// snapshot: use it for a report, never for a decision that must be exact.
func (q *Queue[T]) All() iter.Seq[T] {
	return q.list.All()
}

// Values returns the values in the queue as a slice, oldest first, and
// dequeues none of them. It has the same best-effort reading as All.
func (q *Queue[T]) Values() []T {
	return q.list.Values()
}

// TryAdd enqueues value and always reports true. A lock-free queue has no
// bound, so the add never fails.
func (q *Queue[T]) TryAdd(value T) bool {
	return q.list.TryAdd(value)
}

// TryTake removes the oldest value and returns it. It is [Queue.TryDequeue]
// under the name a general collection uses.
func (q *Queue[T]) TryTake() (T, bool) {
	return q.list.TryTake()
}
