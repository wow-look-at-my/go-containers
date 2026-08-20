package concurrentmap

import (
	"fmt"
	"math/bits"
	"runtime"
	"slices"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- construction ----------

func TestNewDefaultShardCount(t *testing.T) {
	m := New[string, int]()
	n := len(m.shards)

	assert.GreaterOrEqual(t, n, minShards)
	assert.LessOrEqual(t, n, maxShards)
	assert.Equal(t, 1, bits.OnesCount(uint(n)), "shard count must be a power of two")
	assert.Equal(t, shardCount(4*runtime.GOMAXPROCS(0)), n)
	assert.Equal(t, uint64(n-1), m.mask)
}

func TestShardCount(t *testing.T) {
	cases := []struct {
		want int
		got  int
	}{
		{-100, 8},
		{0, 8},
		{1, 8},
		{8, 8},
		{9, 16},
		{16, 16},
		{100, 128},
		{1024, 1024},
		{5000, 1024},
	}
	for _, c := range cases {
		assert.Equal(t, c.got, shardCount(c.want), "shardCount(%d)", c.want)
	}
}

func TestWithConcurrency(t *testing.T) {
	m := New[int, int](WithConcurrency(100))
	assert.Len(t, m.shards, 128)

	m = New[int, int](WithConcurrency(1))
	assert.Len(t, m.shards, minShards)

	m = New[int, int](WithConcurrency(1 << 20))
	assert.Len(t, m.shards, maxShards)
}

func TestWithCapacity(t *testing.T) {
	m := New[int, int](WithConcurrency(8), WithCapacity(800))
	require.Len(t, m.shards, 8)
	for i := range 1000 {
		m.Store(i, i)
	}
	assert.Equal(t, 1000, m.Len())
}

func TestOptionsCombine(t *testing.T) {
	m := New[int, int](WithCapacity(64), WithConcurrency(32))
	assert.Len(t, m.shards, 32)
	m.Store(1, 1)
	assert.Equal(t, 1, m.Len())
}

// TestShardIsCacheLineSized guards the padding. A shard that shrinks below the
// target shares a cache line with the next shard.
func TestShardIsCacheLineSized(t *testing.T) {
	assert.EqualValues(t, shardBytes, unsafe.Sizeof(shard[int, int]{}))
	assert.EqualValues(t, shardBytes, unsafe.Sizeof(shard[string, []byte]{}))
}

// TestShardsAreDistinctAllocations compares addresses. Two shards hold equal
// contents at the start, so a value comparison proves nothing here.
func TestShardsAreDistinctAllocations(t *testing.T) {
	m := New[int, int](WithConcurrency(8))
	seen := make([]uintptr, 0, len(m.shards))
	for _, s := range m.shards {
		addr := uintptr(unsafe.Pointer(s))
		assert.NotContains(t, seen, addr)
		seen = append(seen, addr)
	}
	assert.Len(t, seen, 8)
}

// TestKeysSpreadOverShards proves the mask selects more than one shard.
func TestKeysSpreadOverShards(t *testing.T) {
	m := New[int, int](WithConcurrency(8))
	for i := range 1000 {
		m.Store(i, i)
	}
	used := 0
	for _, s := range m.shards {
		if len(s.m) > 0 {
			used++
		}
	}
	assert.Equal(t, 8, used, "every shard should hold at least one of 1000 keys")
}

// ---------- the zero value ----------

func TestZeroMapPanics(t *testing.T) {
	var m Map[string, int]

	cases := map[string]func(){
		"Store":         func() { m.Store("a", 1) },
		"Load":          func() { m.Load("a") },
		"Contains":      func() { m.Contains("a") },
		"TryAdd":        func() { m.TryAdd("a", 1) },
		"LoadOrStore":   func() { m.LoadOrStore("a", 1) },
		"LoadOrCompute": func() { m.LoadOrCompute("a", func(string) int { return 1 }) },
		"AddOrUpdate":   func() { m.AddOrUpdate("a", 1, func(string, int) int { return 1 }) },
		"Compute":       func() { m.Compute("a", func(int, bool) (int, bool) { return 1, false }) },
		"Delete":        func() { m.Delete("a") },
		"LoadAndDelete": func() { m.LoadAndDelete("a") },
		"Len":           func() { m.Len() },
		"IsEmpty":       func() { m.IsEmpty() },
		"Clear":         func() { m.Clear() },
		"ToMap":         func() { m.ToMap() },
		"String":        func() { _ = m.String() },
		"All": func() {
			for range m.All() {
			}
		},
		"Keys": func() {
			for range m.Keys() {
			}
		},
		"Values": func() {
			for range m.Values() {
			}
		},
		"CompareAndSwap": func() { CompareAndSwap(&m, "a", 1, 2) },
		"CompareAndDel":  func() { CompareAndDelete(&m, "a", 1) },
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			assert.PanicsWithValue(t, errZeroMap, call)
		})
	}
}

// ---------- single-key operations ----------

func TestStoreLoadContains(t *testing.T) {
	m := New[string, int]()

	_, ok := m.Load("missing")
	assert.False(t, ok)
	assert.False(t, m.Contains("missing"))

	m.Store("a", 1)
	v, ok := m.Load("a")
	assert.True(t, ok)
	assert.Equal(t, 1, v)
	assert.True(t, m.Contains("a"))

	m.Store("a", 2)
	v, ok = m.Load("a")
	assert.True(t, ok)
	assert.Equal(t, 2, v)
	assert.Equal(t, 1, m.Len())
}

func TestTryAdd(t *testing.T) {
	m := New[string, int]()

	assert.True(t, m.TryAdd("a", 1))
	assert.False(t, m.TryAdd("a", 2))

	v, ok := m.Load("a")
	assert.True(t, ok)
	assert.Equal(t, 1, v, "TryAdd must not overwrite")
}

func TestLoadOrStore(t *testing.T) {
	m := New[string, int]()

	v, loaded := m.LoadOrStore("a", 1)
	assert.Equal(t, 1, v)
	assert.False(t, loaded)

	v, loaded = m.LoadOrStore("a", 99)
	assert.Equal(t, 1, v)
	assert.True(t, loaded)
	assert.Equal(t, 1, m.Len())
}

func TestLoadOrCompute(t *testing.T) {
	m := New[string, int]()
	calls := 0

	v, loaded := m.LoadOrCompute("a", func(k string) int {
		calls++
		assert.Equal(t, "a", k)
		return 7
	})
	assert.Equal(t, 7, v)
	assert.False(t, loaded)
	assert.Equal(t, 1, calls)

	v, loaded = m.LoadOrCompute("a", func(string) int {
		calls++
		return 99
	})
	assert.Equal(t, 7, v)
	assert.True(t, loaded)
	assert.Equal(t, 1, calls, "the function must not run for a present key")
}

func TestAddOrUpdate(t *testing.T) {
	m := New[string, int]()
	updates := 0

	update := func(k string, old int) int {
		updates++
		assert.Equal(t, "a", k)
		return old + 10
	}

	assert.Equal(t, 1, m.AddOrUpdate("a", 1, update))
	assert.Equal(t, 0, updates, "the add path must not call update")

	assert.Equal(t, 11, m.AddOrUpdate("a", 1, update))
	assert.Equal(t, 1, updates)

	v, ok := m.Load("a")
	assert.True(t, ok)
	assert.Equal(t, 11, v)
}

func TestComputeInsert(t *testing.T) {
	m := New[string, int]()
	calls := 0

	v, present := m.Compute("a", func(old int, loaded bool) (int, bool) {
		calls++
		assert.Equal(t, 0, old)
		assert.False(t, loaded)
		return 5, false
	})
	assert.Equal(t, 5, v)
	assert.True(t, present)
	assert.Equal(t, 1, calls)
}

func TestComputeUpdate(t *testing.T) {
	m := New[string, int]()
	m.Store("a", 5)

	v, present := m.Compute("a", func(old int, loaded bool) (int, bool) {
		assert.Equal(t, 5, old)
		assert.True(t, loaded)
		return old * 3, false
	})
	assert.Equal(t, 15, v)
	assert.True(t, present)
	assert.Equal(t, 15, must(m.Load("a")))
}

func TestComputeRemove(t *testing.T) {
	m := New[string, int]()
	m.Store("a", 5)

	v, present := m.Compute("a", func(old int, loaded bool) (int, bool) {
		assert.True(t, loaded)
		return old, true
	})
	assert.Equal(t, 0, v, "a removed key reports the zero value")
	assert.False(t, present)
	assert.False(t, m.Contains("a"))
}

func TestComputeRemoveAbsent(t *testing.T) {
	m := New[string, int]()
	v, present := m.Compute("a", func(_ int, loaded bool) (int, bool) {
		assert.False(t, loaded)
		return 0, true
	})
	assert.Equal(t, 0, v)
	assert.False(t, present)
	assert.True(t, m.IsEmpty())
}

func TestDelete(t *testing.T) {
	m := New[string, int]()
	m.Store("a", 1)

	m.Delete("a")
	assert.False(t, m.Contains("a"))

	m.Delete("a")
	assert.Equal(t, 0, m.Len())
}

func TestLoadAndDelete(t *testing.T) {
	m := New[string, int]()
	m.Store("a", 1)

	v, ok := m.LoadAndDelete("a")
	assert.True(t, ok)
	assert.Equal(t, 1, v)

	v, ok = m.LoadAndDelete("a")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

// ---------- whole-map operations ----------

func TestLenIsEmptyClear(t *testing.T) {
	m := New[int, int]()
	assert.True(t, m.IsEmpty())
	assert.Equal(t, 0, m.Len())

	for i := range 500 {
		m.Store(i, i)
	}
	assert.Equal(t, 500, m.Len())
	assert.False(t, m.IsEmpty())

	m.Clear()
	assert.Equal(t, 0, m.Len())
	assert.True(t, m.IsEmpty())

	m.Store(1, 1)
	assert.Equal(t, 1, m.Len())
}

func TestIterators(t *testing.T) {
	m := New[int, int]()
	for i := range 100 {
		m.Store(i, i*2)
	}

	seen := m.ToMap()
	assert.Len(t, seen, 100)
	for i := range 100 {
		assert.Equal(t, i*2, seen[i])
	}

	keys := slices.Sorted(m.Keys())
	require.Len(t, keys, 100)
	assert.Equal(t, 0, keys[0])
	assert.Equal(t, 99, keys[99])

	values := slices.Sorted(m.Values())
	require.Len(t, values, 100)
	assert.Equal(t, 0, values[0])
	assert.Equal(t, 198, values[99])
}

func TestIteratorsStopEarly(t *testing.T) {
	m := New[int, int]()
	for i := range 100 {
		m.Store(i, i)
	}

	count := 0
	for range m.All() {
		count++
		if count == 3 {
			break
		}
	}
	assert.Equal(t, 3, count)

	count = 0
	for range m.Keys() {
		count++
		break
	}
	assert.Equal(t, 1, count)

	count = 0
	for range m.Values() {
		count++
		break
	}
	assert.Equal(t, 1, count)

	// The pooled buffers must survive an early exit.
	assert.Len(t, m.ToMap(), 100)
}

func TestIteratorsOnEmptyMap(t *testing.T) {
	m := New[int, int]()
	assert.Empty(t, m.ToMap())
	assert.Empty(t, slices.Collect(m.Keys()))
	assert.Empty(t, slices.Collect(m.Values()))
}

func TestString(t *testing.T) {
	m := New[string, int]()
	assert.Equal(t, "map[]", m.String())

	m.Store("a", 1)
	m.Store("b", 2)
	assert.Equal(t, "map[a:1 b:2]", m.String())
	assert.Equal(t, "map[a:1 b:2]", fmt.Sprintf("%v", m))
}

// ---------- comparable-value operations ----------

func TestCompareAndSwap(t *testing.T) {
	m := New[string, int]()

	assert.False(t, CompareAndSwap(m, "a", 1, 2), "an absent key must not swap")

	m.Store("a", 1)
	assert.False(t, CompareAndSwap(m, "a", 99, 2), "a mismatch must not swap")
	assert.Equal(t, 1, must(m.Load("a")))

	assert.True(t, CompareAndSwap(m, "a", 1, 2))
	assert.Equal(t, 2, must(m.Load("a")))
}

func TestCompareAndDelete(t *testing.T) {
	m := New[string, int]()

	assert.False(t, CompareAndDelete(m, "a", 1))

	m.Store("a", 1)
	assert.False(t, CompareAndDelete(m, "a", 99))
	assert.True(t, m.Contains("a"))

	assert.True(t, CompareAndDelete(m, "a", 1))
	assert.False(t, m.Contains("a"))
}

// must drops the presence result of a two-result read.
func must[V any](v V, _ bool) V {
	return v
}
