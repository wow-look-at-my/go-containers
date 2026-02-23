package sortedmap

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	m := New[int, string]()
	require.Equal(t, 0, m.Len(), "expected empty map")
	require.True(t, m.IsEmpty(), "expected IsEmpty to return true")
}

func TestPutAndGet(t *testing.T) {
	m := New[int, string]()
	m.Put(3, "three")
	m.Put(1, "one")
	m.Put(2, "two")

	require.Equal(t, 3, m.Len(), "expected 3 entries")
	for _, tc := range []struct {
		key  int
		want string
	}{
		{1, "one"}, {2, "two"}, {3, "three"},
	} {
		v, ok := m.Get(tc.key)
		assert.True(t, ok, "Get(%d): expected ok", tc.key)
		assert.Equal(t, tc.want, v, "Get(%d) = %q, want %q", tc.key, v, tc.want)
	}

	_, ok := m.Get(99)
	assert.False(t, ok, "Get(99) should return false for missing key")
}

func TestPutOverwrite(t *testing.T) {
	m := New[string, int]()
	m.Put("key", 1)
	m.Put("key", 2)
	v, _ := m.Get("key")
	assert.Equal(t, 2, v, "expected overwritten value 2")
	assert.Equal(t, 1, m.Len(), "expected len 1 after overwrite")
}

func TestContains(t *testing.T) {
	m := New[int, string]()
	m.Put(1, "one")
	assert.True(t, m.Contains(1), "expected Contains(1) = true")
	assert.False(t, m.Contains(2), "expected Contains(2) = false")
}

func TestDelete(t *testing.T) {
	m := New[int, string]()
	m.Put(1, "one")
	m.Put(2, "two")
	m.Put(3, "three")

	assert.True(t, m.Delete(2), "Delete(2) should return true")
	assert.False(t, m.Contains(2), "key 2 should be gone after delete")
	assert.Equal(t, 2, m.Len(), "expected len 2")

	assert.False(t, m.Delete(99), "Delete(99) should return false for missing key")
}

func TestDeleteAll(t *testing.T) {
	m := New[int, int]()
	for i := 0; i < 10; i++ {
		m.Put(i, i)
	}
	for i := 0; i < 10; i++ {
		assert.True(t, m.Delete(i), "Delete(%d) should return true", i)
	}
	assert.True(t, m.IsEmpty(), "expected empty map, got len %d", m.Len())
}

func TestDeleteReverseOrder(t *testing.T) {
	m := New[int, int]()
	for i := 0; i < 20; i++ {
		m.Put(i, i)
	}
	for i := 19; i >= 0; i-- {
		assert.True(t, m.Delete(i), "Delete(%d) should return true", i)
	}
	assert.True(t, m.IsEmpty(), "expected empty map, got len %d", m.Len())
}

func TestDeleteEvens(t *testing.T) {
	m := New[int, int]()
	for i := 0; i < 100; i++ {
		m.Put(i, i*10)
	}
	for i := 0; i < 100; i += 2 {
		m.Delete(i)
	}
	require.Equal(t, 50, m.Len(), "expected 50 remaining")
	var keys []int
	for k := range m.Keys() {
		keys = append(keys, k)
	}
	for i, k := range keys {
		assert.Equal(t, i*2+1, k, "keys[%d] = %d, want %d", i, k, i*2+1)
	}
}

func TestClear(t *testing.T) {
	m := New[int, string]()
	m.Put(1, "one")
	m.Put(2, "two")
	m.Clear()
	require.Equal(t, 0, m.Len(), "expected len 0 after clear")
	require.True(t, m.IsEmpty(), "expected IsEmpty after clear")
}

// ---------- ordered operations ----------

func TestMinMax(t *testing.T) {
	m := New[int, string]()

	_, _, ok := m.Min()
	assert.False(t, ok, "Min on empty map should return false")
	_, _, ok = m.Max()
	assert.False(t, ok, "Max on empty map should return false")

	m.Put(5, "five")
	m.Put(1, "one")
	m.Put(9, "nine")
	m.Put(3, "three")

	k, v, ok := m.Min()
	assert.False(t, !ok || k != 1 || v != "one", "Min() = (%d, %q, %v), want (1, \"one\", true)", k, v, ok)
	k, v, ok = m.Max()
	assert.False(t, !ok || k != 9 || v != "nine", "Max() = (%d, %q, %v), want (9, \"nine\", true)", k, v, ok)
}

func TestFloor(t *testing.T) {
	m := New[int, string]()
	m.Put(2, "two")
	m.Put(4, "four")
	m.Put(6, "six")

	tests := []struct {
		key     int
		wantKey int
		wantVal string
		wantOK  bool
	}{
		{1, 0, "", false},    // below all keys
		{2, 2, "two", true},  // exact match
		{3, 2, "two", true},  // between keys
		{4, 4, "four", true}, // exact match
		{5, 4, "four", true}, // between keys
		{6, 6, "six", true},  // exact match
		{99, 6, "six", true}, // above all keys
	}
	for _, tc := range tests {
		k, v, ok := m.Floor(tc.key)
		assert.False(t, ok != tc.wantOK || k != tc.wantKey || v != tc.wantVal,
			"Floor(%d) = (%d, %q, %v), want (%d, %q, %v)",
			tc.key, k, v, ok, tc.wantKey, tc.wantVal, tc.wantOK)
	}
}

func TestCeiling(t *testing.T) {
	m := New[int, string]()
	m.Put(2, "two")
	m.Put(4, "four")
	m.Put(6, "six")

	tests := []struct {
		key     int
		wantKey int
		wantVal string
		wantOK  bool
	}{
		{1, 2, "two", true},  // below all keys
		{2, 2, "two", true},  // exact match
		{3, 4, "four", true}, // between keys
		{4, 4, "four", true}, // exact match
		{5, 6, "six", true},  // between keys
		{6, 6, "six", true},  // exact match
		{99, 0, "", false},   // above all keys
	}
	for _, tc := range tests {
		k, v, ok := m.Ceiling(tc.key)
		assert.False(t, ok != tc.wantOK || k != tc.wantKey || v != tc.wantVal,
			"Ceiling(%d) = (%d, %q, %v), want (%d, %q, %v)",
			tc.key, k, v, ok, tc.wantKey, tc.wantVal, tc.wantOK)
	}
}

// ---------- iteration ----------

func TestAll(t *testing.T) {
	m := New[int, string]()
	m.Put(3, "three")
	m.Put(1, "one")
	m.Put(2, "two")

	var keys []int
	var vals []string
	for k, v := range m.All() {
		keys = append(keys, k)
		vals = append(vals, v)
	}
	assert.True(t, slices.Equal(keys, []int{1, 2, 3}), "All keys = %v, want [1 2 3]", keys)
	assert.True(t, slices.Equal(vals, []string{"one", "two", "three"}), "All vals = %v, want [one two three]", vals)
}

func TestAllEarlyBreak(t *testing.T) {
	m := New[int, int]()
	for i := 0; i < 10; i++ {
		m.Put(i, i)
	}
	count := 0
	for range m.All() {
		count++
		if count == 3 {
			break
		}
	}
	assert.Equal(t, 3, count, "expected 3 iterations before break")
}

func TestKeys(t *testing.T) {
	m := New[int, string]()
	m.Put(5, "five")
	m.Put(1, "one")
	m.Put(3, "three")

	var keys []int
	for k := range m.Keys() {
		keys = append(keys, k)
	}
	assert.True(t, slices.Equal(keys, []int{1, 3, 5}), "Keys = %v, want [1 3 5]", keys)
}

func TestValues(t *testing.T) {
	m := New[int, string]()
	m.Put(2, "two")
	m.Put(1, "one")
	m.Put(3, "three")

	var vals []string
	for v := range m.Values() {
		vals = append(vals, v)
	}
	assert.True(t, slices.Equal(vals, []string{"one", "two", "three"}), "Values = %v, want [one two three]", vals)
}

func TestBackward(t *testing.T) {
	m := New[int, string]()
	m.Put(1, "one")
	m.Put(2, "two")
	m.Put(3, "three")

	var keys []int
	for k, _ := range m.Backward() {
		keys = append(keys, k)
	}
	assert.True(t, slices.Equal(keys, []int{3, 2, 1}), "Backward keys = %v, want [3 2 1]", keys)
}

func TestRange(t *testing.T) {
	m := New[int, string]()
	for i := 1; i <= 10; i++ {
		m.Put(i, fmt.Sprintf("%d", i))
	}

	var keys []int
	for k, _ := range m.Range(3, 7) {
		keys = append(keys, k)
	}
	assert.Equal(t, []int{3, 4, 5, 6, 7}, keys, "Range(3,7) keys")
}

func TestRangeNoResults(t *testing.T) {
	m := New[int, string]()
	m.Put(1, "one")
	m.Put(10, "ten")

	var keys []int
	for k, _ := range m.Range(3, 7) {
		keys = append(keys, k)
	}
	assert.Equal(t, 0, len(keys), "expected no keys in range, got %v", keys)
}

func TestRangeSingleElement(t *testing.T) {
	m := New[int, string]()
	m.Put(5, "five")
	m.Put(1, "one")
	m.Put(9, "nine")

	var keys []int
	for k, _ := range m.Range(5, 5) {
		keys = append(keys, k)
	}
	assert.True(t, slices.Equal(keys, []int{5}), "Range(5,5) = %v, want [5]", keys)
}

// ---------- String ----------

func TestString(t *testing.T) {
	m := New[int, string]()
	m.Put(2, "two")
	m.Put(1, "one")
	expected := "{1: one, 2: two}"
	assert.Equal(t, expected, m.String())
}

func TestStringEmpty(t *testing.T) {
	m := New[int, string]()
	assert.Equal(t, "{}", m.String())
}

// ---------- custom comparator ----------

func TestNewWithCompare(t *testing.T) {
	// Reverse ordering.
	m := NewWithCompare[int, string](func(a, b int) int {
		return cmp.Compare(b, a)
	})
	m.Put(1, "one")
	m.Put(2, "two")
	m.Put(3, "three")

	var keys []int
	for k := range m.Keys() {
		keys = append(keys, k)
	}
	assert.True(t, slices.Equal(keys, []int{3, 2, 1}), "reverse-order Keys = %v, want [3 2 1]", keys)

	k, _, _ := m.Min()
	assert.Equal(t, 3, k, "Min key with reverse comparator")
	k, _, _ = m.Max()
	assert.Equal(t, 1, k, "Max key with reverse comparator")
}

// ---------- string keys ----------

func TestStringKeys(t *testing.T) {
	m := New[string, int]()
	m.Put("banana", 2)
	m.Put("apple", 1)
	m.Put("cherry", 3)

	var keys []string
	for k := range m.Keys() {
		keys = append(keys, k)
	}
	assert.True(t, slices.Equal(keys, []string{"apple", "banana", "cherry"}), "string keys = %v, want [apple banana cherry]", keys)
}

// ---------- empty map edge cases ----------

func TestEmptyMapOperations(t *testing.T) {
	m := New[int, int]()
	assert.False(t, m.Delete(1), "Delete on empty map should return false")
	assert.False(t, m.Contains(1), "Contains on empty map should return false")
	_, ok := m.Get(1)
	assert.False(t, ok, "Get on empty map should return false")

	count := 0
	for range m.All() {
		count++
	}
	assert.Equal(t, 0, count, "All on empty map yielded %d items", count)

	_, _, ok = m.Floor(1)
	assert.False(t, ok, "Floor on empty map should return false")
	_, _, ok = m.Ceiling(1)
	assert.False(t, ok, "Ceiling on empty map should return false")
}

// ---------- single element ----------

func TestSingleElement(t *testing.T) {
	m := New[int, string]()
	m.Put(42, "answer")

	k, v, ok := m.Min()
	assert.False(t, !ok || k != 42 || v != "answer",
		"Min = (%d, %q, %v), want (42, \"answer\", true)", k, v, ok)
	k, v, ok = m.Max()
	assert.False(t, !ok || k != 42 || v != "answer",
		"Max = (%d, %q, %v), want (42, \"answer\", true)", k, v, ok)

	assert.True(t, m.Delete(42), "Delete(42) should return true")
	assert.True(t, m.IsEmpty(), "map should be empty after deleting only element")
}

// ---------- stress test ----------

func TestRandomInsertDelete(t *testing.T) {
	m := New[int, int]()
	ref := make(map[int]int) // reference map

	rng := rand.New(rand.NewPCG(12345, 67890))

	for i := 0; i < 2000; i++ {
		key := rng.IntN(200)
		if rng.IntN(3) == 0 && len(ref) > 0 {
			// Delete.
			m.Delete(key)
			delete(ref, key)
		} else {
			// Insert.
			m.Put(key, key*10)
			ref[key] = key * 10
		}
	}

	require.Equal(t, len(ref), m.Len(), "size mismatch")

	// Verify all reference entries are present.
	for k, v := range ref {
		got, ok := m.Get(k)
		assert.False(t, !ok || got != v,
			"Get(%d) = (%d, %v), want (%d, true)", k, got, ok, v)
	}

	// Verify sorted order.
	var prev int
	first := true
	for k := range m.Keys() {
		require.False(t, !first && k <= prev, "keys not sorted: %d after %d", k, prev)
		prev = k
		first = false
	}
}

// ---------- benchmarks ----------

func BenchmarkPut(b *testing.B) {
	m := New[int, int]()
	b.ResetTimer()
	for i := range b.N {
		m.Put(i, i)
	}
}

func BenchmarkGet(b *testing.B) {
	m := New[int, int]()
	for i := range 1000 {
		m.Put(i, i)
	}
	b.ResetTimer()
	for range b.N {
		m.Get(500)
	}
}

func BenchmarkDelete(b *testing.B) {
	for range b.N {
		b.StopTimer()
		m := New[int, int]()
		for i := range 1000 {
			m.Put(i, i)
		}
		b.StartTimer()
		for i := range 1000 {
			m.Delete(i)
		}
	}
}

func BenchmarkIterate(b *testing.B) {
	m := New[int, int]()
	for i := range 1000 {
		m.Put(i, i)
	}
	b.ResetTimer()
	for range b.N {
		for range m.All() {
		}
	}
}
