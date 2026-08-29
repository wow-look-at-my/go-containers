package orderedset

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZeroValueIsUsable(t *testing.T) {
	var s OrderedSet[string]
	assert.True(t, s.IsEmpty())
	assert.Equal(t, 0, s.Len())
	assert.False(t, s.Contains("a"))
	assert.Empty(t, s.Values())

	assert.True(t, s.Add("a"))
	assert.True(t, s.Add("b"))
	assert.Equal(t, []string{"a", "b"}, s.Values())
}

func TestAddKeepsFirstAddedOrder(t *testing.T) {
	s := Of(3, 1, 2)
	assert.Equal(t, []int{3, 1, 2}, s.Values())

	assert.False(t, s.Add(3), "already present")
	assert.Equal(t, []int{3, 1, 2}, s.Values(), "a repeat must not move an element")

	assert.True(t, s.Add(0))
	assert.Equal(t, []int{3, 1, 2, 0}, s.Values())
}

func TestOfDropsRepeats(t *testing.T) {
	s := Of("b", "a", "b", "c", "a")
	assert.Equal(t, []string{"b", "a", "c"}, s.Values())
	assert.Equal(t, 3, s.Len())
}

func TestRemoveKeepsTheOrderOfWhatIsLeft(t *testing.T) {
	s := Of(1, 2, 3, 4, 5)
	s.Remove(2, 4)
	assert.Equal(t, []int{1, 3, 5}, s.Values())
	assert.False(t, s.Contains(2))
}

func TestReAddedElementGoesToTheEnd(t *testing.T) {
	s := Of(1, 2, 3)
	s.Remove(1)
	assert.True(t, s.Add(1))
	assert.Equal(t, []int{2, 3, 1}, s.Values())
}

func TestRemoveIgnoresAbsentElements(t *testing.T) {
	s := Of(1, 2)
	s.Remove(9)
	assert.Equal(t, []int{1, 2}, s.Values())
	assert.Equal(t, 0, s.dead)
}

// Removing most of a large set must reclaim the slots left behind rather than
// growing order without bound.
func TestCompactionReclaimsDeadSlots(t *testing.T) {
	s := New[int](100)
	for i := range 100 {
		s.Add(i)
	}
	for i := range 90 {
		s.Remove(i)
	}
	assert.Equal(t, 10, s.Len())
	assert.Less(t, len(s.order), 100, "order should have been rebuilt")
	assert.Equal(t, []int{90, 91, 92, 93, 94, 95, 96, 97, 98, 99}, s.Values())
}

// Compaction must survive an element that has both a dead slot and a live one.
func TestCompactionWithReAddedElements(t *testing.T) {
	s := New[int]()
	for i := range 20 {
		s.Add(i)
	}
	for i := range 16 {
		s.Remove(i)
		s.Add(i + 100)
	}
	want := []int{16, 17, 18, 19}
	for i := range 16 {
		want = append(want, i+100)
	}
	assert.Equal(t, want, s.Values())
}

func TestClearEmptiesTheSet(t *testing.T) {
	s := Of(1, 2, 3)
	s.Clear()
	assert.True(t, s.IsEmpty())
	assert.Empty(t, s.Values())

	s.Add(7)
	assert.Equal(t, []int{7}, s.Values())
}

func TestCloneIsIndependentAndOrdered(t *testing.T) {
	s := Of(3, 1, 2)
	s.Remove(1)
	c := s.Clone()
	assert.Equal(t, []int{3, 2}, c.Values())

	c.Add(9)
	assert.Equal(t, []int{3, 2}, s.Values(), "the original must not change")
}

func TestContainsHelpers(t *testing.T) {
	s := Of(1, 2, 3)
	assert.True(t, s.ContainsAll(1, 3))
	assert.False(t, s.ContainsAll(1, 9))
	assert.True(t, s.ContainsAny(9, 2))
	assert.False(t, s.ContainsAny(8, 9))
}

func TestAllIteratesInOrderAndStopsEarly(t *testing.T) {
	s := Of(5, 6, 7, 8)
	s.Remove(6)

	assert.Equal(t, []int{5, 7, 8}, slices.Collect(s.All()))

	var seen []int
	for v := range s.All() {
		seen = append(seen, v)
		if len(seen) == 2 {
			break
		}
	}
	assert.Equal(t, []int{5, 7}, seen)
}

func TestBackwardIteratesInReverseAndStopsEarly(t *testing.T) {
	s := Of(5, 6, 7, 8)
	s.Remove(6)

	assert.Equal(t, []int{8, 7, 5}, slices.Collect(s.Backward()))

	var seen []int
	for v := range s.Backward() {
		seen = append(seen, v)
		break
	}
	assert.Equal(t, []int{8}, seen)
}

func TestStringShowsOrder(t *testing.T) {
	assert.Equal(t, "[3 1 2]", Of(3, 1, 2).String())
}

func TestUnionTakesTheLeftOrderFirst(t *testing.T) {
	a := Of(3, 1)
	b := Of(1, 4, 2)
	assert.Equal(t, []int{3, 1, 4, 2}, a.Union(b).Values())
}

func TestIntersectionKeepsTheLeftOrder(t *testing.T) {
	a := Of(4, 3, 2, 1)
	b := Of(1, 2)
	assert.Equal(t, []int{2, 1}, a.Intersection(b).Values())
}

func TestDifferenceKeepsTheLeftOrder(t *testing.T) {
	a := Of(4, 3, 2, 1)
	b := Of(3, 1)
	assert.Equal(t, []int{4, 2}, a.Difference(b).Values())
}

func TestSymmetricDifferenceIsLeftThenRight(t *testing.T) {
	a := Of(4, 3, 2)
	b := Of(3, 9, 8)
	assert.Equal(t, []int{4, 2, 9, 8}, a.SymmetricDifference(b).Values())
}

func TestAlgebraSkipsDeadSlots(t *testing.T) {
	a := Of(1, 2, 3)
	a.Remove(2)
	b := Of(2, 3)

	assert.Equal(t, []int{1, 3, 2}, a.Union(b).Values())
	assert.Equal(t, []int{3}, a.Intersection(b).Values())
	assert.Equal(t, []int{1}, a.Difference(b).Values())
	assert.Equal(t, []int{1, 2}, a.SymmetricDifference(b).Values())
}

func TestSubsetAndSupersetRelations(t *testing.T) {
	a := Of(1, 2)
	b := Of(2, 1, 3)

	assert.True(t, a.IsSubsetOf(b))
	assert.False(t, b.IsSubsetOf(a))
	assert.True(t, b.IsSupersetOf(a))
	assert.True(t, a.IsProperSubsetOf(b))
	assert.False(t, b.IsProperSubsetOf(b))
	assert.True(t, b.IsProperSupersetOf(a))
}

func TestEqualIgnoresOrderAndEqualOrderedDoesNot(t *testing.T) {
	a := Of(1, 2, 3)
	b := Of(3, 2, 1)

	assert.True(t, a.Equal(b))
	assert.False(t, a.EqualOrdered(b))
	assert.True(t, a.EqualOrdered(Of(1, 2, 3)))
	assert.False(t, a.EqualOrdered(Of(1, 2)))
	assert.False(t, a.Equal(Of(1, 2, 9)))
}

func TestIsDisjoint(t *testing.T) {
	assert.True(t, Of(1, 2).IsDisjoint(Of(3, 4, 5)))
	assert.False(t, Of(1, 2).IsDisjoint(Of(2, 3)))
}

func TestAddSetAppendsInTheOtherOrder(t *testing.T) {
	a := Of(1, 2)
	a.AddSet(Of(2, 5, 4))
	assert.Equal(t, []int{1, 2, 5, 4}, a.Values())
}

func TestRemoveSetFromTheLargerSide(t *testing.T) {
	a := Of(1, 2, 3, 4)
	a.RemoveSet(Of(2, 4))
	assert.Equal(t, []int{1, 3}, a.Values())
}

func TestRemoveSetFromTheSmallerSide(t *testing.T) {
	a := Of(1, 2)
	a.RemoveSet(Of(2, 3, 4, 5, 6))
	assert.Equal(t, []int{1}, a.Values())
}

func TestRetainAll(t *testing.T) {
	a := Of(4, 3, 2, 1)
	a.RetainAll(Of(1, 3))
	assert.Equal(t, []int{3, 1}, a.Values())
}

func TestJSONRoundTripPreservesOrder(t *testing.T) {
	s := Of("c", "a", "b")
	data, err := s.MarshalJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `["c","a","b"]`, string(data))

	var back OrderedSet[string]
	require.NoError(t, back.UnmarshalJSON(data))
	assert.Equal(t, []string{"c", "a", "b"}, back.Values())
}

func TestUnmarshalJSONReplacesAndDeduplicates(t *testing.T) {
	s := Of(1, 2, 3)
	require.NoError(t, s.UnmarshalJSON([]byte(`[9,8,9]`)))
	assert.Equal(t, []int{9, 8}, s.Values())

	require.NoError(t, s.UnmarshalJSON([]byte(`[]`)))
	assert.True(t, s.IsEmpty())

	assert.Error(t, s.UnmarshalJSON([]byte(`{"not":"an array"}`)))
}
