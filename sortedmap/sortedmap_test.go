package sortedmap

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

func TestNew(t *testing.T) {
	m := New[int, string]()
	if m.Len() != 0 {
		t.Fatalf("expected empty map, got len %d", m.Len())
	}
	if !m.IsEmpty() {
		t.Fatal("expected IsEmpty to return true")
	}
}

func TestPutAndGet(t *testing.T) {
	m := New[int, string]()
	m.Put(3, "three")
	m.Put(1, "one")
	m.Put(2, "two")

	if m.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", m.Len())
	}
	for _, tc := range []struct {
		key  int
		want string
	}{
		{1, "one"}, {2, "two"}, {3, "three"},
	} {
		v, ok := m.Get(tc.key)
		if !ok {
			t.Errorf("Get(%d): expected ok", tc.key)
		}
		if v != tc.want {
			t.Errorf("Get(%d) = %q, want %q", tc.key, v, tc.want)
		}
	}

	_, ok := m.Get(99)
	if ok {
		t.Error("Get(99) should return false for missing key")
	}
}

func TestPutOverwrite(t *testing.T) {
	m := New[string, int]()
	m.Put("key", 1)
	m.Put("key", 2)
	v, _ := m.Get("key")
	if v != 2 {
		t.Errorf("expected overwritten value 2, got %d", v)
	}
	if m.Len() != 1 {
		t.Errorf("expected len 1 after overwrite, got %d", m.Len())
	}
}

func TestContains(t *testing.T) {
	m := New[int, string]()
	m.Put(1, "one")
	if !m.Contains(1) {
		t.Error("expected Contains(1) = true")
	}
	if m.Contains(2) {
		t.Error("expected Contains(2) = false")
	}
}

func TestDelete(t *testing.T) {
	m := New[int, string]()
	m.Put(1, "one")
	m.Put(2, "two")
	m.Put(3, "three")

	if !m.Delete(2) {
		t.Error("Delete(2) should return true")
	}
	if m.Contains(2) {
		t.Error("key 2 should be gone after delete")
	}
	if m.Len() != 2 {
		t.Errorf("expected len 2, got %d", m.Len())
	}

	if m.Delete(99) {
		t.Error("Delete(99) should return false for missing key")
	}
}

func TestDeleteAll(t *testing.T) {
	m := New[int, int]()
	for i := 0; i < 10; i++ {
		m.Put(i, i)
	}
	for i := 0; i < 10; i++ {
		if !m.Delete(i) {
			t.Errorf("Delete(%d) should return true", i)
		}
	}
	if !m.IsEmpty() {
		t.Errorf("expected empty map, got len %d", m.Len())
	}
}

func TestDeleteReverseOrder(t *testing.T) {
	m := New[int, int]()
	for i := 0; i < 20; i++ {
		m.Put(i, i)
	}
	for i := 19; i >= 0; i-- {
		if !m.Delete(i) {
			t.Errorf("Delete(%d) should return true", i)
		}
	}
	if !m.IsEmpty() {
		t.Errorf("expected empty map, got len %d", m.Len())
	}
}

func TestDeleteEvens(t *testing.T) {
	m := New[int, int]()
	for i := 0; i < 100; i++ {
		m.Put(i, i*10)
	}
	for i := 0; i < 100; i += 2 {
		m.Delete(i)
	}
	if m.Len() != 50 {
		t.Fatalf("expected 50 remaining, got %d", m.Len())
	}
	var keys []int
	for k := range m.Keys() {
		keys = append(keys, k)
	}
	for i, k := range keys {
		if k != i*2+1 {
			t.Errorf("keys[%d] = %d, want %d", i, k, i*2+1)
		}
	}
}

func TestClear(t *testing.T) {
	m := New[int, string]()
	m.Put(1, "one")
	m.Put(2, "two")
	m.Clear()
	if m.Len() != 0 {
		t.Fatalf("expected len 0 after clear, got %d", m.Len())
	}
	if !m.IsEmpty() {
		t.Fatal("expected IsEmpty after clear")
	}
}

// ---------- ordered operations ----------

func TestMinMax(t *testing.T) {
	m := New[int, string]()

	_, _, ok := m.Min()
	if ok {
		t.Error("Min on empty map should return false")
	}
	_, _, ok = m.Max()
	if ok {
		t.Error("Max on empty map should return false")
	}

	m.Put(5, "five")
	m.Put(1, "one")
	m.Put(9, "nine")
	m.Put(3, "three")

	k, v, ok := m.Min()
	if !ok || k != 1 || v != "one" {
		t.Errorf("Min() = (%d, %q, %v), want (1, \"one\", true)", k, v, ok)
	}
	k, v, ok = m.Max()
	if !ok || k != 9 || v != "nine" {
		t.Errorf("Max() = (%d, %q, %v), want (9, \"nine\", true)", k, v, ok)
	}
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
		{1, 0, "", false},     // below all keys
		{2, 2, "two", true},   // exact match
		{3, 2, "two", true},   // between keys
		{4, 4, "four", true},  // exact match
		{5, 4, "four", true},  // between keys
		{6, 6, "six", true},   // exact match
		{99, 6, "six", true},  // above all keys
	}
	for _, tc := range tests {
		k, v, ok := m.Floor(tc.key)
		if ok != tc.wantOK || k != tc.wantKey || v != tc.wantVal {
			t.Errorf("Floor(%d) = (%d, %q, %v), want (%d, %q, %v)",
				tc.key, k, v, ok, tc.wantKey, tc.wantVal, tc.wantOK)
		}
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
		{1, 2, "two", true},   // below all keys
		{2, 2, "two", true},   // exact match
		{3, 4, "four", true},  // between keys
		{4, 4, "four", true},  // exact match
		{5, 6, "six", true},   // between keys
		{6, 6, "six", true},   // exact match
		{99, 0, "", false},    // above all keys
	}
	for _, tc := range tests {
		k, v, ok := m.Ceiling(tc.key)
		if ok != tc.wantOK || k != tc.wantKey || v != tc.wantVal {
			t.Errorf("Ceiling(%d) = (%d, %q, %v), want (%d, %q, %v)",
				tc.key, k, v, ok, tc.wantKey, tc.wantVal, tc.wantOK)
		}
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
	if !slices.Equal(keys, []int{1, 2, 3}) {
		t.Errorf("All keys = %v, want [1 2 3]", keys)
	}
	if !slices.Equal(vals, []string{"one", "two", "three"}) {
		t.Errorf("All vals = %v, want [one two three]", vals)
	}
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
	if count != 3 {
		t.Errorf("expected 3 iterations before break, got %d", count)
	}
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
	if !slices.Equal(keys, []int{1, 3, 5}) {
		t.Errorf("Keys = %v, want [1 3 5]", keys)
	}
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
	if !slices.Equal(vals, []string{"one", "two", "three"}) {
		t.Errorf("Values = %v, want [one two three]", vals)
	}
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
	if !slices.Equal(keys, []int{3, 2, 1}) {
		t.Errorf("Backward keys = %v, want [3 2 1]", keys)
	}
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
	if !slices.Equal(keys, []int{3, 4, 5, 6, 7}) {
		t.Errorf("Range(3,7) keys = %v, want [3 4 5 6 7]", keys)
	}
}

func TestRangeNoResults(t *testing.T) {
	m := New[int, string]()
	m.Put(1, "one")
	m.Put(10, "ten")

	var keys []int
	for k, _ := range m.Range(3, 7) {
		keys = append(keys, k)
	}
	if len(keys) != 0 {
		t.Errorf("expected no keys in range, got %v", keys)
	}
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
	if !slices.Equal(keys, []int{5}) {
		t.Errorf("Range(5,5) = %v, want [5]", keys)
	}
}

// ---------- String ----------

func TestString(t *testing.T) {
	m := New[int, string]()
	m.Put(2, "two")
	m.Put(1, "one")
	expected := "{1: one, 2: two}"
	if got := m.String(); got != expected {
		t.Errorf("String() = %q, want %q", got, expected)
	}
}

func TestStringEmpty(t *testing.T) {
	m := New[int, string]()
	if got := m.String(); got != "{}" {
		t.Errorf("String() = %q, want \"{}\"", got)
	}
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
	if !slices.Equal(keys, []int{3, 2, 1}) {
		t.Errorf("reverse-order Keys = %v, want [3 2 1]", keys)
	}

	k, _, _ := m.Min()
	if k != 3 {
		t.Errorf("Min key with reverse comparator = %d, want 3", k)
	}
	k, _, _ = m.Max()
	if k != 1 {
		t.Errorf("Max key with reverse comparator = %d, want 1", k)
	}
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
	if !slices.Equal(keys, []string{"apple", "banana", "cherry"}) {
		t.Errorf("string keys = %v, want [apple banana cherry]", keys)
	}
}

// ---------- empty map edge cases ----------

func TestEmptyMapOperations(t *testing.T) {
	m := New[int, int]()
	if m.Delete(1) {
		t.Error("Delete on empty map should return false")
	}
	if m.Contains(1) {
		t.Error("Contains on empty map should return false")
	}
	_, ok := m.Get(1)
	if ok {
		t.Error("Get on empty map should return false")
	}

	count := 0
	for range m.All() {
		count++
	}
	if count != 0 {
		t.Errorf("All on empty map yielded %d items", count)
	}

	_, _, ok = m.Floor(1)
	if ok {
		t.Error("Floor on empty map should return false")
	}
	_, _, ok = m.Ceiling(1)
	if ok {
		t.Error("Ceiling on empty map should return false")
	}
}

// ---------- single element ----------

func TestSingleElement(t *testing.T) {
	m := New[int, string]()
	m.Put(42, "answer")

	k, v, ok := m.Min()
	if !ok || k != 42 || v != "answer" {
		t.Errorf("Min = (%d, %q, %v), want (42, \"answer\", true)", k, v, ok)
	}
	k, v, ok = m.Max()
	if !ok || k != 42 || v != "answer" {
		t.Errorf("Max = (%d, %q, %v), want (42, \"answer\", true)", k, v, ok)
	}

	if !m.Delete(42) {
		t.Error("Delete(42) should return true")
	}
	if !m.IsEmpty() {
		t.Error("map should be empty after deleting only element")
	}
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

	if m.Len() != len(ref) {
		t.Fatalf("size mismatch: got %d, want %d", m.Len(), len(ref))
	}

	// Verify all reference entries are present.
	for k, v := range ref {
		got, ok := m.Get(k)
		if !ok || got != v {
			t.Errorf("Get(%d) = (%d, %v), want (%d, true)", k, got, ok, v)
		}
	}

	// Verify sorted order.
	var prev int
	first := true
	for k := range m.Keys() {
		if !first && k <= prev {
			t.Fatalf("keys not sorted: %d after %d", k, prev)
		}
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
