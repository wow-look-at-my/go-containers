package concurrentqueue

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZeroValueIsUsable(t *testing.T) {
	var q Queue[int]
	assert.True(t, q.IsEmpty())
	assert.Zero(t, q.Len())
	_, ok := q.TryDequeue()
	assert.False(t, ok)
	q.Enqueue(1)
	assert.Equal(t, 1, q.Len())
}

func TestFIFOOrder(t *testing.T) {
	q := New[string]()
	q.Enqueue("a")
	q.Enqueue("b")
	q.Enqueue("c")
	require.Equal(t, 3, q.Len())

	for _, want := range []string{"a", "b", "c"} {
		v, ok := q.TryDequeue()
		require.True(t, ok)
		assert.Equal(t, want, v)
	}
	_, ok := q.TryDequeue()
	assert.False(t, ok)
}

func TestEnqueueRangeAndDequeueRange(t *testing.T) {
	q := New[int]()
	q.EnqueueRange(1, 2, 3, 4)
	buf := make([]int, 2)
	n := q.TryDequeueRange(buf)
	require.Equal(t, 2, n)
	assert.Equal(t, []int{1, 2}, buf)
	assert.Equal(t, []int{3, 4}, q.Values())
}

func TestTryPeekLeavesTheValue(t *testing.T) {
	q := New[int]()
	_, ok := q.TryPeek()
	assert.False(t, ok)

	q.Enqueue(5)
	v, ok := q.TryPeek()
	require.True(t, ok)
	assert.Equal(t, 5, v)
	assert.Equal(t, 1, q.Len())
}

func TestClear(t *testing.T) {
	q := New[int]()
	q.EnqueueRange(1, 2, 3)
	q.Clear()
	assert.True(t, q.IsEmpty())

	q.Enqueue(9)
	v, ok := q.TryDequeue()
	require.True(t, ok)
	assert.Equal(t, 9, v)
}

func TestValuesAndAllAgree(t *testing.T) {
	q := New[int]()
	q.EnqueueRange(1, 2, 3, 4)

	var viaAll []int
	for v := range q.All() {
		viaAll = append(viaAll, v)
	}
	assert.Equal(t, q.Values(), viaAll)
}

func TestTryAddTryTake(t *testing.T) {
	q := New[int]()
	assert.True(t, q.TryAdd(1))
	assert.True(t, q.TryAdd(2))
	v, ok := q.TryTake()
	require.True(t, ok)
	assert.Equal(t, 1, v)
}

// Many goroutines enqueue and dequeue at once; the queue must never lose or
// duplicate a value, and Len must settle back to zero.
func TestConcurrentProducersAndConsumers(t *testing.T) {
	q := New[int]()
	const n = 500

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			q.Enqueue(v)
		}(i)
	}
	wg.Wait()
	require.Equal(t, n, q.Len())

	var mu sync.Mutex
	seen := make(map[int]bool, n)
	var consumers sync.WaitGroup
	for i := 0; i < n; i++ {
		consumers.Add(1)
		go func() {
			defer consumers.Done()
			v, ok := q.TryDequeue()
			require.True(t, ok)
			mu.Lock()
			seen[v] = true
			mu.Unlock()
		}()
	}
	consumers.Wait()

	assert.Len(t, seen, n, "every enqueued value was dequeued exactly once")
	assert.True(t, q.IsEmpty())
}
