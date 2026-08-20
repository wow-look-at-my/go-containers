package concurrentbag

import (
	"slices"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockingAddAndTake(t *testing.T) {
	b := NewBlocking[int]()
	require.NoError(t, b.Add(t.Context(), 1))
	require.NoError(t, b.Add(t.Context(), 2))
	assert.Equal(t, 2, b.Len())
	assert.Equal(t, Unbounded, b.Cap())

	// The bag keeps no order, so the test collects both and compares sets.
	got := make([]int, 0, 2)
	for range 2 {
		v, err := b.Take(t.Context())
		require.NoError(t, err)
		got = append(got, v)
	}
	assert.ElementsMatch(t, []int{1, 2}, got)
	assert.True(t, b.IsEmpty())
}

func TestBlockingBagCapacityStopsAdd(t *testing.T) {
	b := NewBlocking[int](WithCapacity(2))
	assert.Equal(t, 2, b.Cap())

	assert.True(t, b.TryAdd(1))
	assert.True(t, b.TryAdd(2))
	assert.False(t, b.TryAdd(3), "a full bag must refuse the add")

	_, ok := b.TryTake()
	require.True(t, ok)
	assert.True(t, b.TryAdd(3), "a free slot must let the add through")
}

func TestBlockingBagTakeWaitsForAdd(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBlocking[int]()

		var got int
		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			got, err = b.Take(context.Background())
		}()

		synctest.Wait()
		select {
		case <-done:
			t.Fatal("Take returned before any add")
		default:
		}

		b.TryAdd(42)
		<-done
		require.NoError(t, err)
		assert.Equal(t, 42, got)
	})
}

func TestBlockingBagAddWaitsForTake(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBlocking[int](WithCapacity(1))
		require.NoError(t, b.Add(context.Background(), 1))

		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			err = b.Add(context.Background(), 2)
		}()

		synctest.Wait()
		select {
		case <-done:
			t.Fatal("Add returned while the bag was full")
		default:
		}

		_, ok := b.TryTake()
		require.True(t, ok)
		<-done
		require.NoError(t, err)
	})
}

func TestBlockingBagContextEndsTheWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBlocking[int]()
		ctx, cancel := context.WithCancel(context.Background())

		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, err = b.Take(ctx)
		}()

		synctest.Wait()
		cancel()
		<-done
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestBlockingBagCompleteAdding(t *testing.T) {
	b := NewBlocking[int]()
	require.NoError(t, b.Add(t.Context(), 1))
	b.CompleteAdding()

	assert.True(t, b.IsAddingCompleted())
	assert.False(t, b.IsCompleted(), "a bag with elements left is not complete")
	assert.ErrorIs(t, b.Add(t.Context(), 2), ErrCompleted)

	v, err := b.Take(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, v)

	_, err = b.Take(t.Context())
	assert.ErrorIs(t, err, ErrCompleted)
	assert.True(t, b.IsCompleted())
}

func TestBlockingBagConsumeDrainsEverything(t *testing.T) {
	b := NewBlocking[int]()
	for i := range 50 {
		require.True(t, b.TryAdd(i))
	}
	b.CompleteAdding()

	got := make([]int, 0, 50)
	for v := range b.Consume(t.Context()) {
		got = append(got, v)
	}
	want := make([]int, 50)
	for i := range want {
		want[i] = i
	}
	assert.ElementsMatch(t, want, got)
	assert.True(t, b.IsCompleted())
}

func TestBlockingBagReadsDoNotRemove(t *testing.T) {
	b := NewBlocking[int]()
	for i := range 3 {
		require.True(t, b.TryAdd(i))
	}

	_, ok := b.TryPeek()
	require.True(t, ok)
	assert.ElementsMatch(t, []int{0, 1, 2}, b.Values())
	assert.ElementsMatch(t, []int{0, 1, 2}, slices.Collect(b.All()))
	assert.Equal(t, 3, b.Len())
}

// The bound must hold under real contention, and no element may be lost.
func TestBlockingBagBoundedProducerConsumer(t *testing.T) {
	const (
		producers   = 4
		consumers   = 4
		perProducer = 1000
		capacity    = 16
	)

	b := NewBlocking[int](WithCapacity(capacity))
	ctx := t.Context()

	var overCapacity atomic.Bool
	var produced sync.WaitGroup
	for p := range producers {
		produced.Go(func() {
			for i := range perProducer {
				if err := b.Add(ctx, p*perProducer+i); err != nil {
					return
				}
				if b.Len() > capacity {
					overCapacity.Store(true)
				}
			}
		})
	}

	seen := make([]atomic.Bool, producers*perProducer)
	var taken atomic.Int64
	var consumed sync.WaitGroup
	for range consumers {
		consumed.Go(func() {
			for v := range b.Consume(ctx) {
				assert.False(t, seen[v].Swap(true), "value %d came out twice", v)
				taken.Add(1)
			}
		})
	}

	produced.Wait()
	b.CompleteAdding()
	consumed.Wait()

	assert.False(t, overCapacity.Load(), "the bag held more than its capacity")
	assert.Equal(t, int64(producers*perProducer), taken.Load())
	assert.True(t, b.IsCompleted())
	for v := range seen {
		require.True(t, seen[v].Load(), "value %d never came out", v)
	}
}
