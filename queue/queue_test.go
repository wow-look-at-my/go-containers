package queue

import (
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

	v, ok := q.TryDequeue()
	require.True(t, ok)
	assert.Equal(t, "a", v)

	v, ok = q.TryDequeue()
	require.True(t, ok)
	assert.Equal(t, "b", v)

	v, ok = q.TryDequeue()
	require.True(t, ok)
	assert.Equal(t, "c", v)

	_, ok = q.TryDequeue()
	assert.False(t, ok, "the queue is empty")
}

// Growth must preserve FIFO order across the point where the backing array
// wraps and doubles -- the exact bug a naive resize can introduce.
func TestGrowthPreservesOrderAcrossWrap(t *testing.T) {
	q := New[int](4)
	for i := 0; i < 3; i++ {
		q.Enqueue(i)
	}
	// Dequeue two, so head is no longer at index 0.
	v, _ := q.TryDequeue()
	require.Equal(t, 0, v)
	v, _ = q.TryDequeue()
	require.Equal(t, 1, v)

	// Enqueue past the original capacity while head is mid-array, forcing a
	// grow that must re-lay the wrapped elements out correctly.
	for i := 3; i < 10; i++ {
		q.Enqueue(i)
	}
	require.Equal(t, 8, q.Len())

	want := []int{2, 3, 4, 5, 6, 7, 8, 9}
	for _, w := range want {
		v, ok := q.TryDequeue()
		require.True(t, ok)
		assert.Equal(t, w, v)
	}
	_, ok := q.TryDequeue()
	assert.False(t, ok)
}

func TestTryPeekLeavesTheValue(t *testing.T) {
	q := New[int]()
	_, ok := q.TryPeek()
	assert.False(t, ok)

	q.Enqueue(5)
	v, ok := q.TryPeek()
	require.True(t, ok)
	assert.Equal(t, 5, v)
	assert.Equal(t, 1, q.Len(), "peek does not remove")

	v, ok = q.TryDequeue()
	require.True(t, ok)
	assert.Equal(t, 5, v)
}

func TestEnqueueRange(t *testing.T) {
	q := New[int]()
	q.EnqueueRange(1, 2, 3)
	assert.Equal(t, []int{1, 2, 3}, q.Values())
}

func TestClear(t *testing.T) {
	q := New[int]()
	q.EnqueueRange(1, 2, 3)
	q.Clear()
	assert.True(t, q.IsEmpty())
	assert.Empty(t, q.Values())

	// A cleared queue still works.
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

func TestAllStopsEarly(t *testing.T) {
	q := New[int]()
	q.EnqueueRange(1, 2, 3, 4)

	var seen []int
	for v := range q.All() {
		seen = append(seen, v)
		if v == 2 {
			break
		}
	}
	assert.Equal(t, []int{1, 2}, seen)
}

func TestTryAddTryTake(t *testing.T) {
	q := New[int]()
	assert.True(t, q.TryAdd(1))
	assert.True(t, q.TryAdd(2))
	v, ok := q.TryTake()
	require.True(t, ok)
	assert.Equal(t, 1, v)
}
