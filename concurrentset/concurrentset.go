// Package concurrentset provides Set, an unordered collection of unique
// elements safe for concurrent use by multiple goroutines. It is a thin
// wrapper over concurrentmap.Map[T, struct{}]: the shard-and-lock design that
// makes a Map safe for concurrent use already solves the hard part, and a set
// is a map that only ever needs its keys.
package concurrentset

import (
	"fmt"
	"iter"

	"github.com/wow-look-at-my/go-containers/concurrentmap"
)

// Set is an unordered collection of unique elements of type T, safe for
// concurrent use. Zero value: not usable, use New.
type Set[T comparable] struct {
	m *concurrentmap.Map[T, struct{}]
}

// New creates an empty Set. Options are concurrentmap's: WithConcurrency sets
// the shard count, WithCapacity hints the element count.
func New[T comparable](opts ...concurrentmap.Option) *Set[T] {
	return &Set[T]{m: concurrentmap.New[T, struct{}](opts...)}
}

// Add inserts elem into the set. It reports whether the element was newly
// added, or false when it was already present.
func (s *Set[T]) Add(elem T) bool {
	return s.m.TryAdd(elem, struct{}{})
}

// AddRange inserts one or more elements into the set.
func (s *Set[T]) AddRange(elems ...T) {
	for _, e := range elems {
		s.m.TryAdd(e, struct{}{})
	}
}

// Remove deletes one or more elements from the set. It does nothing for an
// element that is absent.
func (s *Set[T]) Remove(elems ...T) {
	for _, e := range elems {
		s.m.Delete(e)
	}
}

// Contains reports whether the set holds elem.
func (s *Set[T]) Contains(elem T) bool {
	return s.m.Contains(elem)
}

// Len returns the number of elements in the set. See concurrentmap.Map.Len
// for what "exact" means while another goroutine is writing.
func (s *Set[T]) Len() int {
	return s.m.Len()
}

// IsEmpty reports whether the set holds no elements.
func (s *Set[T]) IsEmpty() bool {
	return s.m.IsEmpty()
}

// Clear removes every element from the set.
func (s *Set[T]) Clear() {
	s.m.Clear()
}

// All returns an iterator over the set's elements: a snapshot per shard, not
// one point in time for the whole set.
func (s *Set[T]) All() iter.Seq[T] {
	return s.m.Keys()
}

// Values returns a snapshot of the set's elements in indeterminate order.
func (s *Set[T]) Values() []T {
	out := make([]T, 0, s.m.Len())
	for v := range s.All() {
		out = append(out, v)
	}
	return out
}

// String returns a human-readable form of the set.
func (s *Set[T]) String() string {
	return fmt.Sprintf("%v", s.Values())
}
