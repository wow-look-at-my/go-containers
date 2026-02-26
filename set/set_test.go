package set

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sorted helper to make slice comparisons deterministic.
func sorted(s []int) []int {
	slices.Sort(s)
	return s
}

func TestNew(t *testing.T) {
	s := New[int]()
	require.Equal(t, 0, s.Len(), "expected empty set")
}

func TestNewWithCapacity(t *testing.T) {
	s := New[int](100)
	require.Equal(t, 0, s.Len(), "expected empty set")
}

func TestOf(t *testing.T) {
	s := Of(1, 2, 3, 2, 1)
	require.Equal(t, 3, s.Len(), "expected 3 unique elements")
	for _, v := range []int{1, 2, 3} {
		assert.True(t, s.Contains(v), "expected set to contain %d", v)
	}
}

func TestAddRemoveContains(t *testing.T) {
	s := New[string]()
	assert.True(t, s.Add("a"), "expected Add to return true for new element")
	assert.True(t, s.Add("b"), "expected Add to return true for new element")
	assert.True(t, s.Add("c"), "expected Add to return true for new element")
	assert.False(t, s.Add("a"), "expected Add to return false for duplicate element")
	require.Equal(t, 3, s.Len(), "expected 3 elements")
	assert.True(t, s.Contains("b"), "expected set to contain 'b'")
	s.Remove("b")
	assert.False(t, s.Contains("b"), "expected 'b' to be removed")
	assert.True(t, s.Add("b"), "expected Add to return true after element was removed")
	require.Equal(t, 3, s.Len(), "expected 3 elements")
}

func TestAddRange(t *testing.T) {
	s := New[int]()
	s.AddRange(1, 2, 3, 2, 1)
	require.Equal(t, 3, s.Len(), "expected 3 unique elements")
	assert.True(t, s.ContainsAll(1, 2, 3), "expected set to contain all added elements")
}

func TestContainsAll(t *testing.T) {
	s := Of(1, 2, 3, 4, 5)
	assert.True(t, s.ContainsAll(1, 3, 5), "expected ContainsAll to return true for subset")
	assert.False(t, s.ContainsAll(1, 6), "expected ContainsAll to return false when an element is missing")
}

func TestContainsAny(t *testing.T) {
	s := Of(1, 2, 3)
	assert.True(t, s.ContainsAny(5, 3), "expected ContainsAny to return true")
	assert.False(t, s.ContainsAny(7, 8), "expected ContainsAny to return false")
}

func TestIsEmpty(t *testing.T) {
	s := New[int]()
	assert.True(t, s.IsEmpty(), "expected new set to be empty")
	s.Add(1)
	assert.False(t, s.IsEmpty(), "expected set with element to not be empty")
}

func TestClear(t *testing.T) {
	s := Of(1, 2, 3)
	s.Clear()
	require.Equal(t, 0, s.Len(), "expected empty set after clear")
}

func TestClone(t *testing.T) {
	s := Of(1, 2, 3)
	c := s.Clone()
	assert.True(t, s.Equal(c), "clone should equal original")
	c.Add(4)
	assert.False(t, s.Contains(4), "mutating clone should not affect original")
}

func TestValues(t *testing.T) {
	s := Of(3, 1, 2)
	vals := sorted(s.Values())
	expected := []int{1, 2, 3}
	assert.True(t, slices.Equal(vals, expected), "expected %v, got %v", expected, vals)
}

func TestAll(t *testing.T) {
	s := Of(1, 2, 3)
	var collected []int
	for v := range s.All() {
		collected = append(collected, v)
	}
	assert.Equal(t, 3, len(collected), "expected 3 elements from iterator")
}

func TestAllEarlyBreak(t *testing.T) {
	s := Of(1, 2, 3, 4, 5)
	count := 0
	for range s.All() {
		count++
		if count == 2 {
			break
		}
	}
	assert.Equal(t, 2, count, "expected iterator to stop after 2")
}

func TestString(t *testing.T) {
	s := Of(42)
	str := s.String()
	assert.Equal(t, "[42]", str)
}

// ---------- set operations ----------

func TestUnion(t *testing.T) {
	a := Of(1, 2, 3)
	b := Of(3, 4, 5)
	u := a.Union(b)
	expected := []int{1, 2, 3, 4, 5}
	assert.True(t, slices.Equal(sorted(u.Values()), expected), "Union: expected %v, got %v", expected, sorted(u.Values()))
}

func TestUnionSmallFirst(t *testing.T) {
	// Exercise the swap branch: s is bigger than other.
	a := Of(1, 2, 3, 4, 5)
	b := Of(3, 6)
	u := a.Union(b)
	expected := []int{1, 2, 3, 4, 5, 6}
	assert.True(t, slices.Equal(sorted(u.Values()), expected), "Union: expected %v, got %v", expected, sorted(u.Values()))
}

func TestIntersection(t *testing.T) {
	a := Of(1, 2, 3, 4)
	b := Of(3, 4, 5, 6)
	inter := a.Intersection(b)
	expected := []int{3, 4}
	assert.True(t, slices.Equal(sorted(inter.Values()), expected), "Intersection: expected %v, got %v", expected, sorted(inter.Values()))
}

func TestIntersectionEmpty(t *testing.T) {
	a := Of(1, 2)
	b := Of(3, 4)
	inter := a.Intersection(b)
	assert.True(t, inter.IsEmpty(), "expected empty intersection")
}

func TestDifference(t *testing.T) {
	a := Of(1, 2, 3, 4)
	b := Of(3, 4, 5)
	diff := a.Difference(b)
	expected := []int{1, 2}
	assert.True(t, slices.Equal(sorted(diff.Values()), expected), "Difference: expected %v, got %v", expected, sorted(diff.Values()))
}

func TestSymmetricDifference(t *testing.T) {
	a := Of(1, 2, 3)
	b := Of(3, 4, 5)
	sd := a.SymmetricDifference(b)
	expected := []int{1, 2, 4, 5}
	assert.True(t, slices.Equal(sorted(sd.Values()), expected), "SymmetricDifference: expected %v, got %v", expected, sorted(sd.Values()))
}

func TestIsSubsetOf(t *testing.T) {
	a := Of(1, 2)
	b := Of(1, 2, 3, 4)
	assert.True(t, a.IsSubsetOf(b), "expected a to be subset of b")
	assert.False(t, b.IsSubsetOf(a), "expected b to NOT be subset of a")
}

func TestIsSubsetOfSelf(t *testing.T) {
	a := Of(1, 2, 3)
	assert.True(t, a.IsSubsetOf(a), "a set is always a subset of itself")
}

func TestIsSupersetOf(t *testing.T) {
	a := Of(1, 2, 3, 4)
	b := Of(1, 2)
	assert.True(t, a.IsSupersetOf(b), "expected a to be superset of b")
	assert.False(t, b.IsSupersetOf(a), "expected b to NOT be superset of a")
}

func TestIsProperSubsetOf(t *testing.T) {
	a := Of(1, 2)
	b := Of(1, 2, 3)
	assert.True(t, a.IsProperSubsetOf(b), "expected a to be proper subset of b")
	assert.False(t, a.IsProperSubsetOf(a), "a set is NOT a proper subset of itself")
}

func TestIsProperSupersetOf(t *testing.T) {
	a := Of(1, 2, 3)
	b := Of(1, 2)
	assert.True(t, a.IsProperSupersetOf(b), "expected a to be proper superset of b")
	assert.False(t, a.IsProperSupersetOf(a), "a set is NOT a proper superset of itself")
}

func TestEqual(t *testing.T) {
	a := Of(1, 2, 3)
	b := Of(3, 2, 1)
	assert.True(t, a.Equal(b), "expected equal sets")
	b.Add(4)
	assert.False(t, a.Equal(b), "expected unequal sets after adding element")
}

func TestIsDisjoint(t *testing.T) {
	a := Of(1, 2)
	b := Of(3, 4)
	assert.True(t, a.IsDisjoint(b), "expected disjoint sets")
	b.Add(2)
	assert.False(t, a.IsDisjoint(b), "expected non-disjoint sets")
}

// ---------- in-place mutations ----------

func TestAddSet(t *testing.T) {
	a := Of(1, 2)
	b := Of(3, 4)
	a.AddSet(b)
	expected := []int{1, 2, 3, 4}
	assert.True(t, slices.Equal(sorted(a.Values()), expected), "AddSet: expected %v, got %v", expected, sorted(a.Values()))
}

func TestRemoveSet(t *testing.T) {
	a := Of(1, 2, 3, 4)
	b := Of(2, 4)
	a.RemoveSet(b)
	expected := []int{1, 3}
	assert.True(t, slices.Equal(sorted(a.Values()), expected), "RemoveSet: expected %v, got %v", expected, sorted(a.Values()))
}

func TestRemoveSetLargerOther(t *testing.T) {
	// Exercise the branch where |other| > |s|.
	a := Of(1, 2)
	b := Of(1, 2, 3, 4, 5, 6)
	a.RemoveSet(b)
	assert.True(t, a.IsEmpty(), "expected empty set, got %v", a.Values())
}

func TestRetainAll(t *testing.T) {
	a := Of(1, 2, 3, 4, 5)
	b := Of(2, 4, 6)
	a.RetainAll(b)
	expected := []int{2, 4}
	assert.True(t, slices.Equal(sorted(a.Values()), expected), "RetainAll: expected %v, got %v", expected, sorted(a.Values()))
}

// ---------- edge cases ----------

func TestEmptySetOperations(t *testing.T) {
	empty := New[int]()
	full := Of(1, 2, 3)

	assert.True(t, empty.Union(full).Equal(full), "union with empty should return other set")
	assert.True(t, empty.Intersection(full).IsEmpty(), "intersection with empty should be empty")
	assert.True(t, full.Difference(empty).Equal(full), "difference with empty should return original set")
	assert.True(t, empty.Difference(full).IsEmpty(), "empty minus full should be empty")
	assert.True(t, empty.IsSubsetOf(full), "empty set is subset of every set")
	assert.True(t, empty.IsDisjoint(full), "empty set is disjoint with every set")
}

func TestStringTypes(t *testing.T) {
	s := Of("hello", "world")
	assert.Equal(t, 2, s.Len())
	assert.False(t, !s.Contains("hello") || !s.Contains("world"), "missing expected string elements")
}

// ---------- zero-value set ----------

func TestZeroValueAdd(t *testing.T) {
	var s Set[int]
	assert.True(t, s.Add(1), "expected Add to return true for new element on zero-value set")
	s.Add(2)
	s.Add(3)
	require.Equal(t, 3, s.Len(), "expected 3 elements")
	assert.True(t, s.Contains(2), "expected set to contain 2")
}

func TestZeroValueReadOps(t *testing.T) {
	var s Set[int]

	assert.Equal(t, 0, s.Len(), "expected Len 0")
	assert.True(t, s.IsEmpty(), "expected zero-value set to be empty")
	assert.False(t, s.Contains(1), "expected Contains to return false")
	assert.False(t, s.ContainsAll(1, 2), "expected ContainsAll to return false")
	assert.False(t, s.ContainsAny(1, 2), "expected ContainsAny to return false")
	assert.Equal(t, 0, len(s.Values()), "expected empty Values")
	assert.Equal(t, "[]", s.String())
}

func TestZeroValueClone(t *testing.T) {
	var s Set[int]
	c := s.Clone()
	assert.True(t, c.IsEmpty(), "clone of zero-value set should be empty")
	c.Add(1)
	assert.False(t, s.Contains(1), "mutating clone should not affect original")
}

func TestZeroValueClear(t *testing.T) {
	var s Set[int]
	s.Clear() // should not panic
	assert.True(t, s.IsEmpty(), "expected empty set after clear")
}

func TestZeroValueRemove(t *testing.T) {
	var s Set[int]
	s.Remove(1) // should not panic
}

func TestZeroValueAddSet(t *testing.T) {
	var s Set[int]
	other := Of(1, 2, 3)
	s.AddSet(other)
	assert.True(t, s.Equal(other), "expected %v, got %v", other.Values(), s.Values())
}

func TestZeroValueAddSetEmpty(t *testing.T) {
	var s Set[int]
	var other Set[int]
	s.AddSet(other) // should not panic
	assert.True(t, s.IsEmpty(), "expected empty set")
}

func TestZeroValueRemoveSet(t *testing.T) {
	var s Set[int]
	other := Of(1, 2)
	s.RemoveSet(other) // should not panic
}

func TestZeroValueRetainAll(t *testing.T) {
	var s Set[int]
	other := Of(1, 2)
	s.RetainAll(other) // should not panic
	assert.True(t, s.IsEmpty(), "expected empty set")
}

func TestZeroValueSetAlgebra(t *testing.T) {
	var empty Set[int]
	full := Of(1, 2, 3)

	assert.True(t, empty.Union(full).Equal(full), "union of zero-value with full should equal full")
	assert.True(t, full.Union(empty).Equal(full), "union of full with zero-value should equal full")
	assert.True(t, empty.Intersection(full).IsEmpty(), "intersection of zero-value with full should be empty")
	assert.True(t, empty.Difference(full).IsEmpty(), "zero-value minus full should be empty")
	assert.True(t, full.Difference(empty).Equal(full), "full minus zero-value should equal full")
	assert.True(t, empty.SymmetricDifference(full).Equal(full), "symmetric difference of zero-value and full should equal full")
	assert.True(t, empty.IsSubsetOf(full), "zero-value set is subset of every set")
	assert.True(t, full.IsSupersetOf(empty), "full set is superset of zero-value set")
	assert.True(t, empty.IsDisjoint(full), "zero-value set is disjoint with every set")
	assert.True(t, empty.Equal(New[int]()), "zero-value set should equal empty initialized set")
}

func TestZeroValueIterator(t *testing.T) {
	var s Set[int]
	count := 0
	for range s.All() {
		count++
	}
	assert.Equal(t, 0, count, "expected 0 iterations")
}

// ---------- benchmarks ----------

func BenchmarkContains(b *testing.B) {
	s := New[int](1000)
	for i := range 1000 {
		s.Add(i)
	}
	b.ResetTimer()
	for range b.N {
		s.Contains(500)
	}
}

func BenchmarkAdd(b *testing.B) {
	s := New[int](b.N)
	b.ResetTimer()
	for i := range b.N {
		s.Add(i)
	}
}

func BenchmarkUnion(b *testing.B) {
	a := New[int](1000)
	c := New[int](1000)
	for i := range 1000 {
		a.Add(i)
		c.Add(i + 500)
	}
	b.ResetTimer()
	for range b.N {
		a.Union(c)
	}
}

func BenchmarkIntersection(b *testing.B) {
	a := New[int](1000)
	c := New[int](1000)
	for i := range 1000 {
		a.Add(i)
		c.Add(i + 500)
	}
	b.ResetTimer()
	for range b.N {
		a.Intersection(c)
	}
}

func BenchmarkDifference(b *testing.B) {
	a := New[int](1000)
	c := New[int](1000)
	for i := range 1000 {
		a.Add(i)
		c.Add(i + 500)
	}
	b.ResetTimer()
	for range b.N {
		a.Difference(c)
	}
}
