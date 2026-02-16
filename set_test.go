package set

import (
	"slices"
	"testing"
)

// sorted helper to make slice comparisons deterministic.
func sorted(s []int) []int {
	slices.Sort(s)
	return s
}

func TestNew(t *testing.T) {
	s := New[int]()
	if s.Len() != 0 {
		t.Fatalf("expected empty set, got len %d", s.Len())
	}
}

func TestNewWithCapacity(t *testing.T) {
	s := New[int](100)
	if s.Len() != 0 {
		t.Fatalf("expected empty set, got len %d", s.Len())
	}
}

func TestOf(t *testing.T) {
	s := Of(1, 2, 3, 2, 1)
	if s.Len() != 3 {
		t.Fatalf("expected 3 unique elements, got %d", s.Len())
	}
	for _, v := range []int{1, 2, 3} {
		if !s.Contains(v) {
			t.Errorf("expected set to contain %d", v)
		}
	}
}

func TestAddRemoveContains(t *testing.T) {
	s := New[string]()
	s.Add("a", "b", "c")
	if s.Len() != 3 {
		t.Fatalf("expected 3 elements, got %d", s.Len())
	}
	if !s.Contains("b") {
		t.Error("expected set to contain 'b'")
	}
	s.Remove("b")
	if s.Contains("b") {
		t.Error("expected 'b' to be removed")
	}
	if s.Len() != 2 {
		t.Fatalf("expected 2 elements, got %d", s.Len())
	}
}

func TestContainsAll(t *testing.T) {
	s := Of(1, 2, 3, 4, 5)
	if !s.ContainsAll(1, 3, 5) {
		t.Error("expected ContainsAll to return true for subset")
	}
	if s.ContainsAll(1, 6) {
		t.Error("expected ContainsAll to return false when an element is missing")
	}
}

func TestContainsAny(t *testing.T) {
	s := Of(1, 2, 3)
	if !s.ContainsAny(5, 3) {
		t.Error("expected ContainsAny to return true")
	}
	if s.ContainsAny(7, 8) {
		t.Error("expected ContainsAny to return false")
	}
}

func TestIsEmpty(t *testing.T) {
	s := New[int]()
	if !s.IsEmpty() {
		t.Error("expected new set to be empty")
	}
	s.Add(1)
	if s.IsEmpty() {
		t.Error("expected set with element to not be empty")
	}
}

func TestClear(t *testing.T) {
	s := Of(1, 2, 3)
	s.Clear()
	if s.Len() != 0 {
		t.Fatalf("expected empty set after clear, got len %d", s.Len())
	}
}

func TestClone(t *testing.T) {
	s := Of(1, 2, 3)
	c := s.Clone()
	if !s.Equal(c) {
		t.Error("clone should equal original")
	}
	c.Add(4)
	if s.Contains(4) {
		t.Error("mutating clone should not affect original")
	}
}

func TestValues(t *testing.T) {
	s := Of(3, 1, 2)
	vals := sorted(s.Values())
	expected := []int{1, 2, 3}
	if !slices.Equal(vals, expected) {
		t.Errorf("expected %v, got %v", expected, vals)
	}
}

func TestAll(t *testing.T) {
	s := Of(1, 2, 3)
	var collected []int
	for v := range s.All() {
		collected = append(collected, v)
	}
	if len(collected) != 3 {
		t.Errorf("expected 3 elements from iterator, got %d", len(collected))
	}
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
	if count != 2 {
		t.Errorf("expected iterator to stop after 2, got %d", count)
	}
}

func TestString(t *testing.T) {
	s := Of(42)
	str := s.String()
	if str != "[42]" {
		t.Errorf("expected '[42]', got %q", str)
	}
}

// ---------- set operations ----------

func TestUnion(t *testing.T) {
	a := Of(1, 2, 3)
	b := Of(3, 4, 5)
	u := a.Union(b)
	expected := []int{1, 2, 3, 4, 5}
	if got := sorted(u.Values()); !slices.Equal(got, expected) {
		t.Errorf("Union: expected %v, got %v", expected, got)
	}
}

func TestUnionSmallFirst(t *testing.T) {
	// Exercise the swap branch: s is bigger than other.
	a := Of(1, 2, 3, 4, 5)
	b := Of(3, 6)
	u := a.Union(b)
	expected := []int{1, 2, 3, 4, 5, 6}
	if got := sorted(u.Values()); !slices.Equal(got, expected) {
		t.Errorf("Union: expected %v, got %v", expected, got)
	}
}

func TestIntersection(t *testing.T) {
	a := Of(1, 2, 3, 4)
	b := Of(3, 4, 5, 6)
	inter := a.Intersection(b)
	expected := []int{3, 4}
	if got := sorted(inter.Values()); !slices.Equal(got, expected) {
		t.Errorf("Intersection: expected %v, got %v", expected, got)
	}
}

func TestIntersectionEmpty(t *testing.T) {
	a := Of(1, 2)
	b := Of(3, 4)
	inter := a.Intersection(b)
	if !inter.IsEmpty() {
		t.Error("expected empty intersection")
	}
}

func TestDifference(t *testing.T) {
	a := Of(1, 2, 3, 4)
	b := Of(3, 4, 5)
	diff := a.Difference(b)
	expected := []int{1, 2}
	if got := sorted(diff.Values()); !slices.Equal(got, expected) {
		t.Errorf("Difference: expected %v, got %v", expected, got)
	}
}

func TestSymmetricDifference(t *testing.T) {
	a := Of(1, 2, 3)
	b := Of(3, 4, 5)
	sd := a.SymmetricDifference(b)
	expected := []int{1, 2, 4, 5}
	if got := sorted(sd.Values()); !slices.Equal(got, expected) {
		t.Errorf("SymmetricDifference: expected %v, got %v", expected, got)
	}
}

func TestIsSubsetOf(t *testing.T) {
	a := Of(1, 2)
	b := Of(1, 2, 3, 4)
	if !a.IsSubsetOf(b) {
		t.Error("expected a to be subset of b")
	}
	if b.IsSubsetOf(a) {
		t.Error("expected b to NOT be subset of a")
	}
}

func TestIsSubsetOfSelf(t *testing.T) {
	a := Of(1, 2, 3)
	if !a.IsSubsetOf(a) {
		t.Error("a set is always a subset of itself")
	}
}

func TestIsSupersetOf(t *testing.T) {
	a := Of(1, 2, 3, 4)
	b := Of(1, 2)
	if !a.IsSupersetOf(b) {
		t.Error("expected a to be superset of b")
	}
	if b.IsSupersetOf(a) {
		t.Error("expected b to NOT be superset of a")
	}
}

func TestIsProperSubsetOf(t *testing.T) {
	a := Of(1, 2)
	b := Of(1, 2, 3)
	if !a.IsProperSubsetOf(b) {
		t.Error("expected a to be proper subset of b")
	}
	if a.IsProperSubsetOf(a) {
		t.Error("a set is NOT a proper subset of itself")
	}
}

func TestIsProperSupersetOf(t *testing.T) {
	a := Of(1, 2, 3)
	b := Of(1, 2)
	if !a.IsProperSupersetOf(b) {
		t.Error("expected a to be proper superset of b")
	}
	if a.IsProperSupersetOf(a) {
		t.Error("a set is NOT a proper superset of itself")
	}
}

func TestEqual(t *testing.T) {
	a := Of(1, 2, 3)
	b := Of(3, 2, 1)
	if !a.Equal(b) {
		t.Error("expected equal sets")
	}
	b.Add(4)
	if a.Equal(b) {
		t.Error("expected unequal sets after adding element")
	}
}

func TestIsDisjoint(t *testing.T) {
	a := Of(1, 2)
	b := Of(3, 4)
	if !a.IsDisjoint(b) {
		t.Error("expected disjoint sets")
	}
	b.Add(2)
	if a.IsDisjoint(b) {
		t.Error("expected non-disjoint sets")
	}
}

// ---------- in-place mutations ----------

func TestAddSet(t *testing.T) {
	a := Of(1, 2)
	b := Of(3, 4)
	a.AddSet(b)
	expected := []int{1, 2, 3, 4}
	if got := sorted(a.Values()); !slices.Equal(got, expected) {
		t.Errorf("AddSet: expected %v, got %v", expected, got)
	}
}

func TestRemoveSet(t *testing.T) {
	a := Of(1, 2, 3, 4)
	b := Of(2, 4)
	a.RemoveSet(b)
	expected := []int{1, 3}
	if got := sorted(a.Values()); !slices.Equal(got, expected) {
		t.Errorf("RemoveSet: expected %v, got %v", expected, got)
	}
}

func TestRemoveSetLargerOther(t *testing.T) {
	// Exercise the branch where |other| > |s|.
	a := Of(1, 2)
	b := Of(1, 2, 3, 4, 5, 6)
	a.RemoveSet(b)
	if !a.IsEmpty() {
		t.Errorf("expected empty set, got %v", a.Values())
	}
}

func TestRetainAll(t *testing.T) {
	a := Of(1, 2, 3, 4, 5)
	b := Of(2, 4, 6)
	a.RetainAll(b)
	expected := []int{2, 4}
	if got := sorted(a.Values()); !slices.Equal(got, expected) {
		t.Errorf("RetainAll: expected %v, got %v", expected, got)
	}
}

// ---------- edge cases ----------

func TestEmptySetOperations(t *testing.T) {
	empty := New[int]()
	full := Of(1, 2, 3)

	if u := empty.Union(full); !u.Equal(full) {
		t.Error("union with empty should return other set")
	}
	if inter := empty.Intersection(full); !inter.IsEmpty() {
		t.Error("intersection with empty should be empty")
	}
	if diff := full.Difference(empty); !diff.Equal(full) {
		t.Error("difference with empty should return original set")
	}
	if diff := empty.Difference(full); !diff.IsEmpty() {
		t.Error("empty minus full should be empty")
	}
	if !empty.IsSubsetOf(full) {
		t.Error("empty set is subset of every set")
	}
	if !empty.IsDisjoint(full) {
		t.Error("empty set is disjoint with every set")
	}
}

func TestStringTypes(t *testing.T) {
	s := Of("hello", "world")
	if s.Len() != 2 {
		t.Errorf("expected 2, got %d", s.Len())
	}
	if !s.Contains("hello") || !s.Contains("world") {
		t.Error("missing expected string elements")
	}
}

// ---------- zero-value set ----------

func TestZeroValueAdd(t *testing.T) {
	var s Set[int]
	s.Add(1, 2, 3)
	if s.Len() != 3 {
		t.Fatalf("expected 3 elements, got %d", s.Len())
	}
	if !s.Contains(2) {
		t.Error("expected set to contain 2")
	}
}

func TestZeroValueReadOps(t *testing.T) {
	var s Set[int]

	if s.Len() != 0 {
		t.Errorf("expected Len 0, got %d", s.Len())
	}
	if !s.IsEmpty() {
		t.Error("expected zero-value set to be empty")
	}
	if s.Contains(1) {
		t.Error("expected Contains to return false")
	}
	if s.ContainsAll(1, 2) {
		t.Error("expected ContainsAll to return false")
	}
	if s.ContainsAny(1, 2) {
		t.Error("expected ContainsAny to return false")
	}
	if v := s.Values(); len(v) != 0 {
		t.Errorf("expected empty Values, got %v", v)
	}
	if str := s.String(); str != "[]" {
		t.Errorf("expected '[]', got %q", str)
	}
}

func TestZeroValueClone(t *testing.T) {
	var s Set[int]
	c := s.Clone()
	if !c.IsEmpty() {
		t.Error("clone of zero-value set should be empty")
	}
	c.Add(1)
	if s.Contains(1) {
		t.Error("mutating clone should not affect original")
	}
}

func TestZeroValueClear(t *testing.T) {
	var s Set[int]
	s.Clear() // should not panic
	if !s.IsEmpty() {
		t.Error("expected empty set after clear")
	}
}

func TestZeroValueRemove(t *testing.T) {
	var s Set[int]
	s.Remove(1) // should not panic
}

func TestZeroValueAddSet(t *testing.T) {
	var s Set[int]
	other := Of(1, 2, 3)
	s.AddSet(other)
	if !s.Equal(other) {
		t.Errorf("expected %v, got %v", other.Values(), s.Values())
	}
}

func TestZeroValueAddSetEmpty(t *testing.T) {
	var s Set[int]
	var other Set[int]
	s.AddSet(other) // should not panic
	if !s.IsEmpty() {
		t.Error("expected empty set")
	}
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
	if !s.IsEmpty() {
		t.Error("expected empty set")
	}
}

func TestZeroValueSetAlgebra(t *testing.T) {
	var empty Set[int]
	full := Of(1, 2, 3)

	if u := empty.Union(full); !u.Equal(full) {
		t.Error("union of zero-value with full should equal full")
	}
	if u := full.Union(empty); !u.Equal(full) {
		t.Error("union of full with zero-value should equal full")
	}
	if inter := empty.Intersection(full); !inter.IsEmpty() {
		t.Error("intersection of zero-value with full should be empty")
	}
	if diff := empty.Difference(full); !diff.IsEmpty() {
		t.Error("zero-value minus full should be empty")
	}
	if diff := full.Difference(empty); !diff.Equal(full) {
		t.Error("full minus zero-value should equal full")
	}
	if sd := empty.SymmetricDifference(full); !sd.Equal(full) {
		t.Error("symmetric difference of zero-value and full should equal full")
	}
	if !empty.IsSubsetOf(full) {
		t.Error("zero-value set is subset of every set")
	}
	if !full.IsSupersetOf(empty) {
		t.Error("full set is superset of zero-value set")
	}
	if !empty.IsDisjoint(full) {
		t.Error("zero-value set is disjoint with every set")
	}
	if !empty.Equal(New[int]()) {
		t.Error("zero-value set should equal empty initialized set")
	}
}

func TestZeroValueIterator(t *testing.T) {
	var s Set[int]
	count := 0
	for range s.All() {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 iterations, got %d", count)
	}
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
