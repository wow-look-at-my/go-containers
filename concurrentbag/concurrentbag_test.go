package concurrentbag

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sizeProbe avoids copying a value that holds atomics.
var sizeProbe shard[int]

// drain takes from the bag until it reports empty.
func drain[T any](b *Bag[T]) []T {
	out := make([]T, 0, b.Len())
	for {
		v, ok := b.TryTake()
		if !ok {
			return out
		}
		out = append(out, v)
	}
}

// counts returns how often each value appears.
func counts(values []int) map[int]int {
	c := make(map[int]int, len(values))
	for _, v := range values {
		c[v]++
	}
	return c
}

func TestShardFillsOneCacheLine(t *testing.T) {
	require.Equal(t, uintptr(cacheLine), unsafe.Sizeof(sizeProbe), "a shard must fill exactly one cache line")
}

func TestNewShardCount(t *testing.T) {
	b := New[int]()
	require.GreaterOrEqual(t, len(b.shards), minShards, "expected at least the floor of shards")
	require.GreaterOrEqual(t, len(b.shards), runtime.GOMAXPROCS(0), "expected at least GOMAXPROCS shards")
	require.Zero(t, len(b.shards)&(len(b.shards)-1), "expected a power of two shard count")
	require.Equal(t, uint64(len(b.shards)-1), b.mask, "expected the mask to select every shard")
	require.True(t, b.IsEmpty(), "expected a new bag to be empty")
}

func TestWithConcurrency(t *testing.T) {
	b := New[int](WithConcurrency(64))
	require.Equal(t, 256, len(b.shards), "expected 4 shards per expected goroutine")

	small := New[int](WithConcurrency(1))
	require.Equal(t, minShards, len(small.shards), "expected the floor to apply")

	odd := New[int](WithConcurrency(3))
	require.Equal(t, 16, len(odd.shards), "expected the count to round up to a power of two")
}

func TestWithConcurrencyIgnoresNonPositive(t *testing.T) {
	want := len(New[int]().shards)
	assert.Equal(t, want, len(New[int](WithConcurrency(0)).shards), "expected zero to have no effect")
	assert.Equal(t, want, len(New[int](WithConcurrency(-5)).shards), "expected a negative value to have no effect")
}

func TestAddAndTryTake(t *testing.T) {
	b := New[string]()
	b.Add("a")
	b.Add("b")
	require.Equal(t, 2, b.Len(), "expected 2 values")

	got := drain(b)
	assert.ElementsMatch(t, []string{"a", "b"}, got, "expected both values back")
	require.Zero(t, b.Len(), "expected an empty bag after the drain")
}

func TestBagKeepsDuplicates(t *testing.T) {
	b := New[int]()
	b.Add(7)
	b.Add(7)
	b.Add(7)
	require.Equal(t, 3, b.Len(), "expected the bag to keep duplicates")
	assert.Equal(t, map[int]int{7: 3}, counts(drain(b)), "expected 3 copies back")
}

func TestTryTakeEmpty(t *testing.T) {
	b := New[int]()
	v, ok := b.TryTake()
	assert.False(t, ok, "expected no value from an empty bag")
	assert.Zero(t, v, "expected the zero value")
}

func TestTryAdd(t *testing.T) {
	b := New[int]()
	assert.True(t, b.TryAdd(1), "expected TryAdd to report true")
	assert.True(t, b.TryAdd(2), "expected TryAdd to report true")
	require.Equal(t, 2, b.Len(), "expected 2 values")
	assert.Equal(t, map[int]int{1: 1, 2: 1}, counts(drain(b)), "expected both values back")
}

func TestAddRange(t *testing.T) {
	b := New[int]()
	b.AddRange(1, 2, 3, 3)
	require.Equal(t, 4, b.Len(), "expected 4 values")
	assert.Equal(t, map[int]int{1: 1, 2: 1, 3: 2}, counts(drain(b)), "expected every value back")
}

func TestAddRangeEmpty(t *testing.T) {
	b := New[int]()
	b.AddRange()
	assert.True(t, b.IsEmpty(), "expected an empty AddRange to add nothing")
}

func TestAddRangeSharesOneShard(t *testing.T) {
	b := New[int]()
	b.AddRange(1, 2, 3, 4, 5)
	occupied := 0
	for i := range b.shards {
		if b.shards[i].top.Load() != nil {
			occupied++
		}
	}
	assert.Equal(t, 1, occupied, "expected one AddRange to fill exactly one shard")
}

func TestTryTakeRange(t *testing.T) {
	b := New[int]()
	b.AddRange(1, 2, 3, 4, 5)

	buf := make([]int, 3)
	require.Equal(t, 3, b.TryTakeRange(buf), "expected a full buffer")
	require.Equal(t, 2, b.Len(), "expected 2 values left")

	rest := make([]int, 10)
	n := b.TryTakeRange(rest)
	require.Equal(t, 2, n, "expected the rest of the bag")
	assert.Equal(t, map[int]int{1: 1, 2: 1, 3: 1, 4: 1, 5: 1}, counts(append(buf, rest[:n]...)), "expected every value once")
	assert.True(t, b.IsEmpty(), "expected an empty bag")
}

func TestTryTakeRangeEmptyCases(t *testing.T) {
	b := New[int]()
	assert.Zero(t, b.TryTakeRange(make([]int, 4)), "expected nothing from an empty bag")
	assert.Zero(t, b.TryTakeRange(nil), "expected nothing for an empty buffer")
	b.Add(1)
	assert.Zero(t, b.TryTakeRange(nil), "expected an empty buffer to take nothing")
	require.Equal(t, 1, b.Len(), "expected the value to stay in the bag")
}

func TestTryPeek(t *testing.T) {
	b := New[int]()
	_, ok := b.TryPeek()
	assert.False(t, ok, "expected no value from an empty bag")

	b.Add(42)
	v, ok := b.TryPeek()
	require.True(t, ok, "expected a value")
	assert.Equal(t, 42, v, "expected the value that was added")
	require.Equal(t, 1, b.Len(), "expected TryPeek to leave the value in the bag")
}

func TestClear(t *testing.T) {
	b := New[int]()
	for i := range 100 {
		b.Add(i)
	}
	require.Equal(t, 100, b.Len(), "expected 100 values")

	b.Clear()
	assert.True(t, b.IsEmpty(), "expected an empty bag after Clear")
	assert.Zero(t, b.Len(), "expected a zero length after Clear")
	_, ok := b.TryTake()
	assert.False(t, ok, "expected no value after Clear")

	b.Add(1)
	assert.Equal(t, 1, b.Len(), "expected the bag to work after Clear")
}

func TestValues(t *testing.T) {
	b := New[int]()
	b.AddRange(1, 2, 3)
	b.Add(4)
	assert.ElementsMatch(t, []int{1, 2, 3, 4}, b.Values(), "expected every value")
	require.Equal(t, 4, b.Len(), "expected Values to remove nothing")

	assert.Empty(t, New[int]().Values(), "expected no values from an empty bag")
}

func TestAll(t *testing.T) {
	b := New[int]()
	b.AddRange(1, 2, 3, 4, 5)

	seen := make([]int, 0, 5)
	for v := range b.All() {
		seen = append(seen, v)
	}
	assert.ElementsMatch(t, []int{1, 2, 3, 4, 5}, seen, "expected every value")
}

func TestAllStopsEarly(t *testing.T) {
	b := New[int]()
	b.AddRange(1, 2, 3, 4, 5)

	seen := 0
	for range b.All() {
		seen++
		break
	}
	assert.Equal(t, 1, seen, "expected the walk to stop on the first break")
	require.Equal(t, 5, b.Len(), "expected the walk to remove nothing")
}

// TestConcurrentAddThenDrain asserts that a parallel add loses no value and
// duplicates no value.
func TestConcurrentAddThenDrain(t *testing.T) {
	const producers, perProducer = 8, 2000

	b := New[int]()
	var wg sync.WaitGroup
	for p := range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perProducer {
				b.Add(p*perProducer + i)
			}
		}()
	}
	wg.Wait()

	require.Equal(t, producers*perProducer, b.Len(), "expected every added value")
	got := drain(b)
	require.Len(t, got, producers*perProducer, "expected the drain to return every value")
	require.Zero(t, b.Len(), "expected exactly zero after a full drain")

	seen := counts(got)
	require.Len(t, seen, producers*perProducer, "expected every value exactly once")
	for v, n := range seen {
		require.Equal(t, 1, n, "expected value %d exactly once", v)
	}
}

// TestConcurrentAddRange asserts the same for the bulk path.
func TestConcurrentAddRange(t *testing.T) {
	const producers, batches, batchSize = 8, 100, 16

	b := New[int]()
	var wg sync.WaitGroup
	for p := range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			batch := make([]int, batchSize)
			for i := range batches {
				base := (p*batches + i) * batchSize
				for j := range batchSize {
					batch[j] = base + j
				}
				b.AddRange(batch...)
			}
		}()
	}
	wg.Wait()

	total := producers * batches * batchSize
	require.Equal(t, total, b.Len(), "expected every added value")
	seen := counts(drain(b))
	require.Len(t, seen, total, "expected every value exactly once")
	for v, n := range seen {
		require.Equal(t, 1, n, "expected value %d exactly once", v)
	}
	require.Zero(t, b.Len(), "expected exactly zero after a full drain")
}

// TestConcurrentAddAndTake runs producers and consumers at the same time. What
// the consumers took, plus what stays in the bag, must equal what went in.
func TestConcurrentAddAndTake(t *testing.T) {
	const producers, consumers, perProducer = 6, 6, 3000

	b := New[int]()
	taken := make([][]int, consumers)
	var live atomic.Int64
	live.Store(producers)

	var wg sync.WaitGroup
	for p := range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer live.Add(-1)
			for i := range perProducer {
				b.Add(p*perProducer + i)
			}
		}()
	}
	for c := range consumers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mine := make([]int, 0, perProducer)
			for {
				v, ok := b.TryTake()
				if ok {
					mine = append(mine, v)
					continue
				}
				if live.Load() == 0 {
					break
				}
				runtime.Gosched()
			}
			taken[c] = mine
		}()
	}
	wg.Wait()

	all := drain(b)
	for _, mine := range taken {
		all = append(all, mine...)
	}
	total := producers * perProducer
	require.Len(t, all, total, "expected nothing lost and nothing duplicated")
	seen := counts(all)
	require.Len(t, seen, total, "expected every value exactly once")
	for v, n := range seen {
		require.Equal(t, 1, n, "expected value %d exactly once", v)
	}
	require.Zero(t, b.Len(), "expected exactly zero after a full drain")
}

// TestConcurrentTakeRange asserts that the bulk take path also takes each
// value exactly once.
func TestConcurrentTakeRange(t *testing.T) {
	const consumers, total = 8, 20000

	b := New[int]()
	for i := range total {
		b.Add(i)
	}

	taken := make([][]int, consumers)
	var wg sync.WaitGroup
	for c := range consumers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]int, 32)
			mine := make([]int, 0, total/consumers)
			for {
				n := b.TryTakeRange(buf)
				if n == 0 {
					break
				}
				mine = append(mine, buf[:n]...)
			}
			taken[c] = mine
		}()
	}
	wg.Wait()

	all := make([]int, 0, total)
	for _, mine := range taken {
		all = append(all, mine...)
	}
	require.Len(t, all, total, "expected every value exactly once")
	require.Len(t, counts(all), total, "expected no duplicates")
	require.Zero(t, b.Len(), "expected exactly zero after a full drain")
}

// TestConcurrentReadersAndWriters walks the bag while other goroutines add and
// take. It asserts that a walk stays safe, not that it sees one instant.
func TestConcurrentReadersAndWriters(t *testing.T) {
	const rounds = 500

	b := New[int]()
	var stop atomic.Bool
	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := range rounds {
			b.Add(i)
			b.AddRange(i, i+1, i+2)
		}
		stop.Store(true)
	}()
	go func() {
		defer wg.Done()
		for !stop.Load() {
			b.TryTake()
			b.TryPeek()
		}
	}()
	go func() {
		defer wg.Done()
		for !stop.Load() {
			for range b.All() {
			}
			_ = b.Values()
			_ = b.Len()
		}
	}()
	wg.Wait()

	require.GreaterOrEqual(t, b.Len(), 0, "expected a length that never falls below zero")
	b.Clear()
	require.Zero(t, b.Len(), "expected exactly zero after Clear")
}
