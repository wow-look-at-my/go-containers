// Package queue provides Queue, a generic first-in-first-out collection for
// single-goroutine use. For concurrent use, see concurrentqueue.
package queue

import "iter"

// minCapacity avoids resizing twice for a queue that stays small.
const minCapacity = 8

// Queue is a first-in-first-out collection, not safe for concurrent use. Zero value ready to use.
type Queue[T any] struct {
	buf   []T
	head  int
	count int
}

// New creates an empty queue with an optional initial capacity hint.
func New[T any](capacity ...int) *Queue[T] {
	var c int
	if len(capacity) > 0 {
		c = capacity[0]
	}
	if c < minCapacity {
		c = minCapacity
	}
	return &Queue[T]{buf: make([]T, c)}
}

// Enqueue adds value to the back of the queue.
func (q *Queue[T]) Enqueue(value T) {
	if q.buf == nil {
		q.buf = make([]T, minCapacity)
	}
	if q.count == len(q.buf) {
		q.grow()
	}
	q.buf[(q.head+q.count)%len(q.buf)] = value
	q.count++
}

// EnqueueRange adds every value to the back of the queue, in the given order.
func (q *Queue[T]) EnqueueRange(values ...T) {
	for _, v := range values {
		q.Enqueue(v)
	}
}

// grow doubles the backing array and re-lays the elements out starting at
// index 0, which is what lets the new array simply be twice the length
// instead of accounting for the old wrap point.
func (q *Queue[T]) grow() {
	size := len(q.buf) * 2
	if size == 0 {
		size = minCapacity
	}
	fresh := make([]T, size)
	for i := 0; i < q.count; i++ {
		fresh[i] = q.buf[(q.head+i)%len(q.buf)]
	}
	q.buf = fresh
	q.head = 0
}

// TryDequeue removes and returns the oldest value. It reports false and the
// zero value of T when the queue is empty.
func (q *Queue[T]) TryDequeue() (T, bool) {
	if q.count == 0 {
		var zero T
		return zero, false
	}
	v := q.buf[q.head]
	var zero T
	q.buf[q.head] = zero // drop the reference so a held pointer can be collected
	q.head = (q.head + 1) % len(q.buf)
	q.count--
	return v, true
}

// TryPeek returns the oldest value and leaves it in the queue. It reports
// false and the zero value of T when the queue is empty.
func (q *Queue[T]) TryPeek() (T, bool) {
	if q.count == 0 {
		var zero T
		return zero, false
	}
	return q.buf[q.head], true
}

// Len returns the number of values in the queue.
func (q *Queue[T]) Len() int {
	return q.count
}

// IsEmpty reports whether the queue holds no values.
func (q *Queue[T]) IsEmpty() bool {
	return q.count == 0
}

// Clear removes every value from the queue.
func (q *Queue[T]) Clear() {
	var zero T
	for i := 0; i < q.count; i++ {
		q.buf[(q.head+i)%len(q.buf)] = zero
	}
	q.head, q.count = 0, 0
}

// Values returns the values in the queue, oldest first.
func (q *Queue[T]) Values() []T {
	out := make([]T, q.count)
	for i := 0; i < q.count; i++ {
		out[i] = q.buf[(q.head+i)%len(q.buf)]
	}
	return out
}

// All returns an iterator over the values in the queue, oldest first.
func (q *Queue[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := 0; i < q.count; i++ {
			if !yield(q.buf[(q.head+i)%len(q.buf)]) {
				return
			}
		}
	}
}

// TryAdd enqueues value and always reports true. It is Enqueue under the
// name a general collection uses.
func (q *Queue[T]) TryAdd(value T) bool {
	q.Enqueue(value)
	return true
}

// TryTake removes and returns the oldest value. It is TryDequeue under the
// name a general collection uses.
func (q *Queue[T]) TryTake() (T, bool) {
	return q.TryDequeue()
}
