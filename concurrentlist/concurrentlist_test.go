package concurrentlist

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZeroValueIsUsable(t *testing.T) {
	var l List[int]
	assert.True(t, l.IsEmpty())
	assert.Equal(t, 0, l.Len())

	_, ok := l.TryTake()
	assert.False(t, ok)

	l.Append(7)
	v, ok := l.TryTake()
	require.True(t, ok)
	assert.Equal(t, 7, v)
}

func TestAppendAndTakeAreFirstInFirstOut(t *testing.T) {
	l := New[int]()
	for i := range 100 {
		l.Append(i)
	}
	require.Equal(t, 100, l.Len())

	for i := range 100 {
		v, ok := l.TryTake()
		require.True(t, ok)
		require.Equal(t, i, v)
	}
	assert.True(t, l.IsEmpty())
}

// The list must keep its order across the segment boundaries, so the count
// here has to exceed the initial segment.
func TestOrderHoldsAcrossSegments(t *testing.T) {
	const n = initialSegmentLen*8 + 3
	l := New[int]()
	for i := range n {
		l.Append(i)
	}
	require.Equal(t, n, l.Len())

	for i := range n {
		v, ok := l.TryTake()
		require.True(t, ok)
		require.Equal(t, i, v)
	}
	_, ok := l.TryTake()
	assert.False(t, ok)
}

func TestAppendRange(t *testing.T) {
	l := New[int]()
	l.AppendRange()
	assert.Equal(t, 0, l.Len())

	want := make([]int, 200)
	for i := range want {
		want[i] = i
	}
	l.AppendRange(want...)
	require.Equal(t, len(want), l.Len())
	assert.Equal(t, want, l.Values())
}

func TestAppendRangeCrossesSegments(t *testing.T) {
	l := New[int]()
	l.Append(-1)
	values := make([]int, initialSegmentLen*3)
	for i := range values {
		values[i] = i
	}
	l.AppendRange(values...)

	v, ok := l.TryTake()
	require.True(t, ok)
	require.Equal(t, -1, v)
	for i := range values {
		v, ok := l.TryTake()
		require.True(t, ok)
		require.Equal(t, i, v)
	}
}

func TestTryTakeRange(t *testing.T) {
	l := New[int]()
	assert.Equal(t, 0, l.TryTakeRange(make([]int, 4)))

	for i := range 100 {
		l.Append(i)
	}

	buf := make([]int, 30)
	assert.Equal(t, 30, l.TryTakeRange(buf))
	for i := range buf {
		assert.Equal(t, i, buf[i])
	}

	big := make([]int, 500)
	assert.Equal(t, 70, l.TryTakeRange(big))
	assert.Equal(t, 30, big[0])
	assert.Equal(t, 99, big[69])
	assert.True(t, l.IsEmpty())
}

func TestTryPeek(t *testing.T) {
	l := New[string]()
	_, ok := l.TryPeek()
	assert.False(t, ok)

	l.Append("first")
	l.Append("second")
	v, ok := l.TryPeek()
	require.True(t, ok)
	assert.Equal(t, "first", v)
	assert.Equal(t, 2, l.Len())
}

func TestClear(t *testing.T) {
	l := New[int]()
	for i := range 500 {
		l.Append(i)
	}
	l.Clear()
	assert.True(t, l.IsEmpty())

	l.Append(1)
	assert.Equal(t, 1, l.Len())
}

func TestAllStopsEarly(t *testing.T) {
	l := New[int]()
	for i := range 10 {
		l.Append(i)
	}

	seen := 0
	for range l.All() {
		seen++
		if seen == 3 {
			break
		}
	}
	assert.Equal(t, 3, seen)
	assert.Equal(t, 10, l.Len())
}

func TestTryAddSatisfiesTheStoreContract(t *testing.T) {
	var store interface {
		TryAdd(int) bool
		TryTake() (int, bool)
		Len() int
	} = New[int]()

	require.True(t, store.TryAdd(42))
	assert.Equal(t, 1, store.Len())
	v, ok := store.TryTake()
	require.True(t, ok)
	assert.Equal(t, 42, v)
}

// Every appended element must come out exactly once, and the takes must
// preserve the order each producer used.
func TestConcurrentAppendAndTakeLosesNothing(t *testing.T) {
	const (
		producers   = 8
		perProducer = 2000
	)

	l := New[int]()
	var wg sync.WaitGroup
	for p := range producers {
		wg.Go(func() {
			for i := range perProducer {
				l.Append(p*perProducer + i)
			}
		})
	}

	var takenCount atomic.Int64
	var done atomic.Bool
	var consumers sync.WaitGroup
	results := make([][]int, 4)
	for c := range 4 {
		consumers.Go(func() {
			for !done.Load() || l.Len() > 0 {
				if v, ok := l.TryTake(); ok {
					results[c] = append(results[c], v)
					takenCount.Add(1)
				}
			}
		})
	}

	wg.Wait()
	done.Store(true)
	consumers.Wait()

	require.Equal(t, int64(producers*perProducer), takenCount.Load())
	require.Equal(t, 0, l.Len())

	seen := make([]bool, producers*perProducer)
	for _, got := range results {
		for _, v := range got {
			require.False(t, seen[v], "value %d came out twice", v)
			seen[v] = true
		}
	}
	for v, ok := range seen {
		require.True(t, ok, "value %d never came out", v)
	}

	// Each producer's own elements must stay in that producer's order.
	for c := range results {
		last := make([]int, producers)
		for i := range last {
			last[i] = -1
		}
		for _, v := range results[c] {
			p := v / perProducer
			require.Greater(t, v, last[p], "producer %d lost its order", p)
			last[p] = v
		}
	}
}

func TestConcurrentBulkOperations(t *testing.T) {
	const (
		producers = 6
		batches   = 200
		batchSize = 16
	)

	l := New[int]()
	var wg sync.WaitGroup
	for p := range producers {
		wg.Go(func() {
			batch := make([]int, batchSize)
			for b := range batches {
				for i := range batch {
					batch[i] = (p*batches+b)*batchSize + i
				}
				l.AppendRange(batch...)
			}
		})
	}
	wg.Wait()

	total := producers * batches * batchSize
	require.Equal(t, total, l.Len())

	var got atomic.Int64
	seen := make([]atomic.Bool, total)
	var takers sync.WaitGroup
	for range 4 {
		takers.Go(func() {
			buf := make([]int, 7)
			for {
				n := l.TryTakeRange(buf)
				if n == 0 {
					return
				}
				for _, v := range buf[:n] {
					assert.False(t, seen[v].Swap(true), "value %d came out twice", v)
				}
				got.Add(int64(n))
			}
		})
	}
	takers.Wait()

	assert.Equal(t, int64(total), got.Load())
	assert.Equal(t, 0, l.Len())
}
