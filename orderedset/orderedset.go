// Package orderedset provides OrderedSet, a set that iterates in the order its
// elements were first added. For an unordered set see set; for key order rather
// than arrival order see sortedmap.
package orderedset

import (
	"fmt"
	"iter"
)

// compactFloor stops a small set rebuilding itself on its first removal.
const compactFloor = 8

// OrderedSet is a collection of unique elements of type T that iterates in
// first-added order. Not safe for concurrent use. Zero value ready to use.
type OrderedSet[T comparable] struct {
	// index maps an element to the slot in order that currently holds it.
	index map[T]int
	// order is append-only; a removal leaves its slot for live to reject.
	order []T
	dead  int
}

// New creates an empty ordered set with an optional initial capacity hint.
func New[T comparable](capacity ...int) OrderedSet[T] {
	var c int
	if len(capacity) > 0 {
		c = capacity[0]
	}
	return OrderedSet[T]{index: make(map[T]int, c), order: make([]T, 0, c)}
}

// Of creates an ordered set holding the given elements, in the order given.
// A repeated element keeps the position of its first appearance.
func Of[T comparable](elems ...T) OrderedSet[T] {
	s := New[T](len(elems))
	s.AddRange(elems...)
	return s
}

// live reports whether slot i currently holds elem.
func (s OrderedSet[T]) live(i int, elem T) bool {
	j, ok := s.index[elem]
	return ok && j == i
}

// Add appends elem if it is absent, and reports whether the set changed. An
// element that is already present keeps the position it already had, which is
// why this looks the element up before inserting it.
func (s *OrderedSet[T]) Add(elem T) bool {
	if _, ok := s.index[elem]; ok {
		return false
	}
	if s.index == nil {
		s.index = make(map[T]int, 1)
	}
	s.index[elem] = len(s.order)
	s.order = append(s.order, elem)
	return true
}

// AddRange appends every element that is not already present, in order.
func (s *OrderedSet[T]) AddRange(elems ...T) {
	for _, e := range elems {
		s.Add(e)
	}
}

// Remove deletes one or more elements.
func (s *OrderedSet[T]) Remove(elems ...T) {
	for _, e := range elems {
		if _, ok := s.index[e]; !ok {
			continue
		}
		delete(s.index, e)
		s.dead++
	}
	s.compact()
}

// compact rebuilds order without its dead slots, once they outnumber the live
// ones. Waiting for that is what makes the cost of a removal constant on
// average rather than linear every time.
func (s *OrderedSet[T]) compact() {
	if s.dead <= len(s.index) || len(s.order) < compactFloor {
		return
	}
	kept := s.order[:0]
	for i, e := range s.order {
		if !s.live(i, e) {
			continue
		}
		s.index[e] = len(kept)
		kept = append(kept, e)
	}
	clear(s.order[len(kept):]) // drop the references the dead slots still hold
	s.order = kept
	s.dead = 0
}

// Contains reports whether the set holds elem.
func (s OrderedSet[T]) Contains(elem T) bool {
	_, ok := s.index[elem]
	return ok
}

// ContainsAll reports whether the set holds every one of the given elements.
func (s OrderedSet[T]) ContainsAll(elems ...T) bool {
	for _, e := range elems {
		if _, ok := s.index[e]; !ok {
			return false
		}
	}
	return true
}

// ContainsAny reports whether the set holds at least one of the given elements.
func (s OrderedSet[T]) ContainsAny(elems ...T) bool {
	for _, e := range elems {
		if _, ok := s.index[e]; ok {
			return true
		}
	}
	return false
}

// Len returns the number of elements in the set.
func (s OrderedSet[T]) Len() int {
	return len(s.index)
}

// IsEmpty reports whether the set holds no elements.
func (s OrderedSet[T]) IsEmpty() bool {
	return len(s.index) == 0
}

// Clear removes every element and keeps the capacity already allocated.
func (s *OrderedSet[T]) Clear() {
	clear(s.index)
	clear(s.order)
	s.order = s.order[:0]
	s.dead = 0
}

// Clone returns a copy holding the same elements in the same order.
func (s OrderedSet[T]) Clone() OrderedSet[T] {
	c := New[T](len(s.index))
	c.appendLive(s)
	return c
}

// appendLive appends every live element of src, in src's order, to a set that
// does not already hold any of them.
func (s *OrderedSet[T]) appendLive(src OrderedSet[T]) {
	for i, e := range src.order {
		if src.live(i, e) {
			s.index[e] = len(s.order)
			s.order = append(s.order, e)
		}
	}
}

// Values returns the elements in first-added order.
func (s OrderedSet[T]) Values() []T {
	if s.dead == 0 {
		out := make([]T, len(s.order))
		copy(out, s.order)
		return out
	}
	out := make([]T, 0, len(s.index))
	for i, e := range s.order {
		if s.live(i, e) {
			out = append(out, e)
		}
	}
	return out
}

// All returns an iterator over the elements in first-added order.
func (s OrderedSet[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for i, e := range s.order {
			if s.live(i, e) && !yield(e) {
				return
			}
		}
	}
}

// Backward returns an iterator over the elements in reverse of first-added order.
func (s OrderedSet[T]) Backward() iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := len(s.order) - 1; i >= 0; i-- {
			if s.live(i, s.order[i]) && !yield(s.order[i]) {
				return
			}
		}
	}
}

// String returns a human-readable representation, in order.
func (s OrderedSet[T]) String() string {
	return fmt.Sprintf("%v", s.Values())
}

// ---------- set-algebraic operations ----------

// Union returns the elements of s in their order, followed by the elements of
// other that s does not hold, in theirs.
func (s OrderedSet[T]) Union(other OrderedSet[T]) OrderedSet[T] {
	out := New[T](s.Len() + other.Len())
	out.appendLive(s)
	for i, e := range other.order {
		if other.live(i, e) {
			out.Add(e)
		}
	}
	return out
}

// Intersection returns the elements s and other share, in s's order.
func (s OrderedSet[T]) Intersection(other OrderedSet[T]) OrderedSet[T] {
	return s.filter(other.Contains)
}

// Difference returns the elements of s that other does not hold, in s's order.
func (s OrderedSet[T]) Difference(other OrderedSet[T]) OrderedSet[T] {
	return s.filter(func(e T) bool { return !other.Contains(e) })
}

// filter returns the elements of s that keep reports true for, in s's order.
func (s OrderedSet[T]) filter(keep func(T) bool) OrderedSet[T] {
	out := New[T]()
	for i, e := range s.order {
		if s.live(i, e) && keep(e) {
			out.index[e] = len(out.order)
			out.order = append(out.order, e)
		}
	}
	return out
}

// SymmetricDifference returns the elements exactly one of s and other holds:
// s's in s's order, then other's in theirs.
func (s OrderedSet[T]) SymmetricDifference(other OrderedSet[T]) OrderedSet[T] {
	out := s.Difference(other)
	for i, e := range other.order {
		if other.live(i, e) && !s.Contains(e) {
			out.Add(e)
		}
	}
	return out
}

// IsSubsetOf reports whether every element of s is also in other. Order plays
// no part: these are set relations, not sequence ones.
func (s OrderedSet[T]) IsSubsetOf(other OrderedSet[T]) bool {
	if s.Len() > other.Len() {
		return false
	}
	for e := range s.index {
		if _, ok := other.index[e]; !ok {
			return false
		}
	}
	return true
}

// IsSupersetOf reports whether s holds every element of other.
func (s OrderedSet[T]) IsSupersetOf(other OrderedSet[T]) bool {
	return other.IsSubsetOf(s)
}

// IsProperSubsetOf reports whether s is a subset of other and the two differ.
func (s OrderedSet[T]) IsProperSubsetOf(other OrderedSet[T]) bool {
	return s.Len() < other.Len() && s.IsSubsetOf(other)
}

// IsProperSupersetOf reports whether s is a superset of other and the two differ.
func (s OrderedSet[T]) IsProperSupersetOf(other OrderedSet[T]) bool {
	return other.IsProperSubsetOf(s)
}

// Equal reports whether s and other hold the same elements, in any order.
// EqualOrdered is the comparison that takes order into account.
func (s OrderedSet[T]) Equal(other OrderedSet[T]) bool {
	return s.Len() == other.Len() && s.IsSubsetOf(other)
}

// EqualOrdered reports whether s and other hold the same elements in the same
// order.
func (s OrderedSet[T]) EqualOrdered(other OrderedSet[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	next, stop := iter.Pull(other.All())
	defer stop()
	for e := range s.All() {
		o, ok := next()
		if !ok || o != e {
			return false
		}
	}
	return true
}

// IsDisjoint reports whether s and other share no elements.
func (s OrderedSet[T]) IsDisjoint(other OrderedSet[T]) bool {
	small, big := s, other
	if small.Len() > big.Len() {
		small, big = big, small
	}
	for e := range small.index {
		if _, ok := big.index[e]; ok {
			return false
		}
	}
	return true
}

// ---------- in-place mutating operations ----------

// AddSet appends every element of other that s does not already hold, in
// other's order.
func (s *OrderedSet[T]) AddSet(other OrderedSet[T]) {
	for i, e := range other.order {
		if other.live(i, e) {
			s.Add(e)
		}
	}
}

// RemoveSet removes every element of other from s.
func (s *OrderedSet[T]) RemoveSet(other OrderedSet[T]) {
	if s.Len() < other.Len() {
		s.retain(func(e T) bool { return !other.Contains(e) })
		return
	}
	for e := range other.index {
		if _, ok := s.index[e]; ok {
			delete(s.index, e)
			s.dead++
		}
	}
	s.compact()
}

// RetainAll removes every element of s that other does not hold.
func (s *OrderedSet[T]) RetainAll(other OrderedSet[T]) {
	s.retain(other.Contains)
}

// retain drops every element keep reports false for. Deleting from a map
// while ranging it is defined, so this needs no scratch slice.
func (s *OrderedSet[T]) retain(keep func(T) bool) {
	for e := range s.index {
		if !keep(e) {
			delete(s.index, e)
			s.dead++
		}
	}
	s.compact()
}
