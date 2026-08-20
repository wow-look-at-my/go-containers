package concurrentlist

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockingAppendAndTake(t *testing.T) {
	b := NewBlocking[int]()
	require.NoError(t, b.Append(t.Context(), 1))
	require.NoError(t, b.Append(t.Context(), 2))
	assert.Equal(t, 2, b.Len())
	assert.Equal(t, Unbounded, b.Cap())

	v, err := b.Take(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, v)
	v, err = b.Take(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, v)
	assert.True(t, b.IsEmpty())
}

func TestBlockingCapacityStopsAppend(t *testing.T) {
	b := NewBlocking[int](WithCapacity(2))
	assert.Equal(t, 2, b.Cap())

	assert.True(t, b.TryAppend(1))
	assert.True(t, b.TryAppend(2))
	assert.False(t, b.TryAppend(3), "a full list must refuse the append")

	_, ok := b.TryTake()
	require.True(t, ok)
	assert.True(t, b.TryAppend(3), "a free slot must let the append through")
}

// A take on an empty list waits until an append delivers an element.
func TestBlockingTakeWaitsForAppend(t *testing.T) {
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
			t.Fatal("Take returned before any append")
		default:
		}

		b.TryAppend(42)
		<-done
		require.NoError(t, err)
		assert.Equal(t, 42, got)
	})
}

// An append on a full list waits until a take frees a slot.
func TestBlockingAppendWaitsForTake(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBlocking[int](WithCapacity(1))
		require.NoError(t, b.Append(context.Background(), 1))

		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			err = b.Append(context.Background(), 2)
		}()

		synctest.Wait()
		select {
		case <-done:
			t.Fatal("Append returned while the list was full")
		default:
		}

		v, ok := b.TryTake()
		require.True(t, ok)
		require.Equal(t, 1, v)

		<-done
		require.NoError(t, err)
		assert.Equal(t, 1, b.Len())
	})
}

func TestBlockingContextEndsTheWait(t *testing.T) {
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

func TestBlockingCompleteAddingWakesTakers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBlocking[int]()

		errs := make([]error, 3)
		var wg sync.WaitGroup
		for i := range errs {
			wg.Go(func() {
				_, errs[i] = b.Take(context.Background())
			})
		}

		synctest.Wait()
		assert.False(t, b.IsAddingCompleted())
		b.CompleteAdding()
		wg.Wait()

		for i, err := range errs {
			assert.ErrorIs(t, err, ErrCompleted, "taker %d", i)
		}
		assert.True(t, b.IsCompleted())
	})
}

// The elements already in the list must still come out after CompleteAdding.
func TestBlockingCompleteAddingDrainsFirst(t *testing.T) {
	b := NewBlocking[int]()
	require.NoError(t, b.Append(t.Context(), 1))
	require.NoError(t, b.Append(t.Context(), 2))
	b.CompleteAdding()

	assert.True(t, b.IsAddingCompleted())
	assert.False(t, b.IsCompleted(), "a list with elements left is not complete")

	v, err := b.Take(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, v)
	v, err = b.Take(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, v)

	_, err = b.Take(t.Context())
	assert.ErrorIs(t, err, ErrCompleted)
	assert.True(t, b.IsCompleted())
}

func TestBlockingAppendAfterCompleteAdding(t *testing.T) {
	b := NewBlocking[int](WithCapacity(4))
	b.CompleteAdding()

	assert.ErrorIs(t, b.Append(t.Context(), 1), ErrCompleted)
	assert.False(t, b.TryAppend(1))
	assert.Equal(t, 0, b.Len())

	b.CompleteAdding() // A second call must change nothing.
	assert.True(t, b.IsCompleted())
}

// A producer blocked on a full list must wake up when the adds complete.
func TestBlockingCompleteAddingWakesAppenders(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBlocking[int](WithCapacity(1))
		require.NoError(t, b.Append(context.Background(), 1))

		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			err = b.Append(context.Background(), 2)
		}()

		synctest.Wait()
		b.CompleteAdding()
		<-done
		assert.ErrorIs(t, err, ErrCompleted)
	})
}

func TestBlockingConsume(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBlocking[int]()

		var got []int
		done := make(chan struct{})
		go func() {
			defer close(done)
			for v := range b.Consume(context.Background()) {
				got = append(got, v)
			}
		}()

		for i := range 5 {
			require.True(t, b.TryAppend(i))
		}
		synctest.Wait()
		b.CompleteAdding()
		<-done

		assert.Equal(t, []int{0, 1, 2, 3, 4}, got)
	})
}

func TestBlockingConsumeStopsEarly(t *testing.T) {
	b := NewBlocking[int]()
	for i := range 5 {
		require.True(t, b.TryAppend(i))
	}

	seen := 0
	for range b.Consume(t.Context()) {
		seen++
		if seen == 2 {
			break
		}
	}
	assert.Equal(t, 2, seen)
	assert.Equal(t, 3, b.Len(), "a stopped consumer must leave the rest")
}

func TestBlockingValuesDoesNotRemove(t *testing.T) {
	b := NewBlocking[int]()
	for i := range 4 {
		require.True(t, b.TryAppend(i))
	}
	assert.Equal(t, []int{0, 1, 2, 3}, b.Values())
	assert.Equal(t, 4, b.Len())
}

// The bound must hold under real contention, and no element may be lost.
func TestBlockingBoundedProducerConsumer(t *testing.T) {
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
				if err := b.Append(ctx, p*perProducer+i); err != nil {
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
				if seen[v].Swap(true) {
					t.Errorf("value %d came out twice", v)
				}
				taken.Add(1)
			}
		})
	}

	produced.Wait()
	b.CompleteAdding()
	consumed.Wait()

	assert.False(t, overCapacity.Load(), "the list held more than its capacity")
	assert.Equal(t, int64(producers*perProducer), taken.Load())
	assert.True(t, b.IsCompleted())
	for v := range seen {
		require.True(t, seen[v].Load(), "value %d never came out", v)
	}
}
