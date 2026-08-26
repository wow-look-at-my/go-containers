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

// Stack is a lock-free Treiber stack: every op is a CAS on one atomic
// pointer. Zero value ready to use; do not copy after first use.
type Stack[T any] struct {
	top atomic.Pointer[node[T]]

	// Push adds BEFORE its CAS, Pop subtracts AFTER -- so length never undercounts.
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
//
// One allocation holds the whole range. The cost is that the last node of a
// range keeps the memory of the whole range, until a pop takes that one too.
func (s *Stack[T]) PushRange(values ...T) {
	if len(values) == 0 {
		return
	}

	// Build the chain first, so a retry costs one CAS, not one per value.
	nodes := make([]node[T], len(values))
	for i, v := range values {
		nodes[i].value = v
		if i > 0 {
			nodes[i].next = &nodes[i-1]
		}
	}
	head, tail := &nodes[len(nodes)-1], &nodes[0]

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
		// No ABA: nodes are never recycled, so this live reference pins old's address.
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

		// Safe without pointer stability: the collector keeps every reached node alive, even popped ones.
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

// TryPeek returns the top value without removing it; false when empty. Another goroutine can pop it first.
func (s *Stack[T]) TryPeek() (T, bool) {
	if top := s.top.Load(); top != nil {
		return top.value, true
	}
	var zero T
	return zero, false
}

// Len is a snapshot; a push in flight counts from the moment it starts, so it never undercounts.
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

	// The walk is what keeps Len correct: no cheaper count of the taken nodes exists.
	var taken int64
	for cur := old; cur != nil; cur = cur.next {
		taken++
	}
	s.length.Add(-taken)
}

// Values returns the values top-down: a snapshot of one chain, not one
// instant -- a value popped mid-walk can still be returned.
func (s *Stack[T]) Values() []T {
	out := make([]T, 0, s.Len())
	for cur := s.top.Load(); cur != nil; cur = cur.next {
		out = append(out, cur.value)
	}
	return out
}

// All iterates the values top-down, loading top when the loop starts; same
// snapshot limits as [Stack.Values].
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
