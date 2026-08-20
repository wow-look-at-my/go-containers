// Package concurrentstack provides a lock-free, last-in-first-out stack for concurrent use.
package concurrentstack

import (
	"iter"
	"sync/atomic"
)

// node is one link of the stack chain. A push sets both fields before it
// links the node in, and nothing writes to a linked node again.
type node[T any] struct {
	value T
	next  *node[T]
}

// Stack is a last-in-first-out collection that many goroutines can use at the
// same time. It is a Treiber stack: every operation is a compare-and-swap on
// one atomic pointer, and there is no mutex on any path.
//
// The zero value is an empty stack ready to use. Do not copy a Stack after
// first use.
type Stack[T any] struct {
	top atomic.Pointer[node[T]]

	// A push adds to length BEFORE its compare-and-swap, and a pop subtracts
	// AFTER its compare-and-swap. This order keeps the counter at or above the
	// true length. The other order lets Clear miss a push in flight and leave
	// the counter one too high forever.
	length atomic.Int64
}

// New returns an empty stack. The zero value works too.
func New[T any]() *Stack[T] {
	return &Stack[T]{}
}

// Push adds value to the top of the stack.
func (s *Stack[T]) Push(value T) {
	n := &node[T]{value: value}
	s.length.Add(1)
	for {
		old := s.top.Load()
		n.next = old
		if s.top.CompareAndSwap(old, n) {
			return
		}
	}
}

// PushRange adds every value to the top of the stack with one
// compare-and-swap. The last value ends on top, so a PushRange of a, b, c pops
// as c, b, a, exactly as three separate calls to Push do.
//
// No other goroutine sees a partial range: the values arrive together or not
// at all.
func (s *Stack[T]) PushRange(values ...T) {
	if len(values) == 0 {
		return
	}

	// Build the chain before the loop. The retry then costs one
	// compare-and-swap, not one per value.
	var head, tail *node[T]
	for _, v := range values {
		head = &node[T]{value: v, next: head}
		if tail == nil {
			tail = head
		}
	}

	s.length.Add(int64(len(values)))
	for {
		old := s.top.Load()
		tail.next = old
		if s.top.CompareAndSwap(old, head) {
			return
		}
	}
}

// TryPop removes the top value and returns it. It reports false and the zero
// value of T when the stack is empty.
func (s *Stack[T]) TryPop() (T, bool) {
	for {
		old := s.top.Load()
		if old == nil {
			var zero T
			return zero, false
		}
		// ABA cannot happen here. The stack never recycles a node, and this
		// goroutine holds a live reference to old, so the collector cannot put
		// a new node at that address while the compare-and-swap runs. Never add
		// a free list or a sync.Pool of nodes: either one brings ABA back.
		if s.top.CompareAndSwap(old, old.next) {
			s.length.Add(-1)
			return old.value, true
		}
	}
}

// TryPopRange removes up to len(buf) values and copies them into buf. It
// returns the number of values it took. The value from the top lands in
// buf[0], so the buffer holds the same order that repeated TryPop calls give.
//
// The whole range comes off with one compare-and-swap. Another goroutine never
// takes a value out of the middle of it.
func (s *Stack[T]) TryPopRange(buf []T) int {
	if len(buf) == 0 {
		return 0
	}
	for {
		old := s.top.Load()
		if old == nil {
			return 0
		}

		// The walk is safe without the pointer being stable: the collector
		// keeps every node this chain reaches alive, even nodes another
		// goroutine already popped.
		count := 1
		last := old
		for count < len(buf) && last.next != nil {
			last = last.next
			count++
		}

		if s.top.CompareAndSwap(old, last.next) {
			cur := old
			for i := range count {
				buf[i] = cur.value
				cur = cur.next
			}
			s.length.Add(int64(-count))
			return count
		}
	}
}

// TryPeek returns the top value and leaves it on the stack. It reports false
// and the zero value of T when the stack is empty. Another goroutine can pop
// that value before the caller reads the result.
func (s *Stack[T]) TryPeek() (T, bool) {
	if top := s.top.Load(); top != nil {
		return top.value, true
	}
	var zero T
	return zero, false
}

// Len returns the number of values on the stack.
//
// The result is a snapshot. Another goroutine can push or pop before the
// caller reads it. A push in flight counts from the moment it starts, so the
// result is never below the true length.
func (s *Stack[T]) Len() int {
	return int(s.length.Load())
}

// IsEmpty reports whether the stack holds no values. It is a snapshot, with
// the same limits as [Stack.Len].
func (s *Stack[T]) IsEmpty() bool {
	return s.Len() == 0
}

// Clear removes every value from the stack. Values that another goroutine
// pushes during the call can survive it.
func (s *Stack[T]) Clear() {
	old := s.top.Swap(nil)

	// The walk is what keeps Len correct: the counter must lose exactly the
	// nodes this call took, and no cheaper count of them exists.
	var taken int64
	for cur := old; cur != nil; cur = cur.next {
		taken++
	}
	s.length.Add(-taken)
}

// Values returns the values on the stack, from the top down.
//
// The result is a snapshot of one chain, not of one point in time. Another
// goroutine can pop a value while the walk runs, and the walk still returns
// that value.
func (s *Stack[T]) Values() []T {
	out := make([]T, 0, s.Len())
	for cur := s.top.Load(); cur != nil; cur = cur.next {
		out = append(out, cur.value)
	}
	return out
}

// All returns an iterator over the values on the stack, from the top down.
// The iterator loads the top when the loop starts. It has the same snapshot
// limits as [Stack.Values].
func (s *Stack[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for cur := s.top.Load(); cur != nil; cur = cur.next {
			if !yield(cur.value) {
				return
			}
		}
	}
}

// TryAdd pushes value and reports true. A lock-free stack has no bound, so the
// add never fails.
func (s *Stack[T]) TryAdd(value T) bool {
	s.Push(value)
	return true
}

// TryTake removes the top value and returns it. It is [Stack.TryPop] under the
// name a general collection uses.
func (s *Stack[T]) TryTake() (T, bool) {
	return s.TryPop()
}
