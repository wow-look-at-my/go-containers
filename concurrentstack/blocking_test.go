package concurrentstack

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

func TestBlockingPushAndPopAreLastInFirstOut(t *testing.T) {
	b := NewBlocking[int]()
	require.NoError(t, b.Push(t.Context(), 1))
	require.NoError(t, b.Push(t.Context(), 2))
	require.NoError(t, b.Push(t.Context(), 3))
	assert.Equal(t, 3, b.Len())
	assert.Equal(t, Unbounded, b.Cap())

	for _, want := range []int{3, 2, 1} {
		v, err := b.Pop(t.Context())
		require.NoError(t, err)
		assert.Equal(t, want, v)
	}
	assert.True(t, b.IsEmpty())
}

func TestBlockingStackCapacityStopsPush(t *testing.T) {
	b := NewBlocking[int](WithCapacity(2))
	assert.Equal(t, 2, b.Cap())

	assert.True(t, b.TryPush(1))
	assert.True(t, b.TryPush(2))
	assert.False(t, b.TryPush(3), "a full stack must refuse the push")

	_, ok := b.TryPop()
	require.True(t, ok)
	assert.True(t, b.TryPush(3), "a free slot must let the push through")
}

func TestBlockingStackPopWaitsForPush(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBlocking[int]()

		var got int
		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			got, err = b.Pop(context.Background())
		}()

		synctest.Wait()
		select {
		case <-done:
			t.Fatal("Pop returned before any push")
		default:
		}

		b.TryPush(42)
		<-done
		require.NoError(t, err)
		assert.Equal(t, 42, got)
	})
}

func TestBlockingStackPushWaitsForPop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBlocking[int](WithCapacity(1))
		require.NoError(t, b.Push(context.Background(), 1))

		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			err = b.Push(context.Background(), 2)
		}()

		synctest.Wait()
		select {
		case <-done:
			t.Fatal("Push returned while the stack was full")
		default:
		}

		_, ok := b.TryPop()
		require.True(t, ok)
		<-done
		require.NoError(t, err)
	})
}

func TestBlockingStackContextEndsTheWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBlocking[int]()
		ctx, cancel := context.WithCancel(context.Background())

		var err error
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, err = b.Pop(ctx)
		}()

		synctest.Wait()
		cancel()
		<-done
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestBlockingStackCompleteAdding(t *testing.T) {
	b := NewBlocking[int]()
	require.NoError(t, b.Push(t.Context(), 1))
	b.CompleteAdding()

	assert.True(t, b.IsAddingCompleted())
	assert.False(t, b.IsCompleted(), "a stack with values left is not complete")
	assert.ErrorIs(t, b.Push(t.Context(), 2), ErrCompleted)

	v, err := b.Pop(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, v)

	_, err = b.Pop(t.Context())
	assert.ErrorIs(t, err, ErrCompleted)
	assert.True(t, b.IsCompleted())
}

func TestBlockingStackConsume(t *testing.T) {
	b := NewBlocking[int]()
	for i := range 4 {
		require.True(t, b.TryPush(i))
	}
	b.CompleteAdding()

	var got []int
	for v := range b.Consume(t.Context()) {
		got = append(got, v)
	}
	assert.Equal(t, []int{3, 2, 1, 0}, got, "the consumer must see last in, first out")
}

func TestBlockingStackReadsDoNotRemove(t *testing.T) {
	b := NewBlocking[int]()
	for i := range 3 {
		require.True(t, b.TryPush(i))
	}

	v, ok := b.TryPeek()
	require.True(t, ok)
	assert.Equal(t, 2, v)
	assert.Equal(t, []int{2, 1, 0}, b.Values())
	assert.Equal(t, []int{2, 1, 0}, slices.Collect(b.All()))
	assert.Equal(t, 3, b.Len())
}

// The bound must hold under real contention, and no value may be lost.
func TestBlockingStackBoundedProducerConsumer(t *testing.T) {
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
				if err := b.Push(ctx, p*perProducer+i); err != nil {
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

	assert.False(t, overCapacity.Load(), "the stack held more than its capacity")
	assert.Equal(t, int64(producers*perProducer), taken.Load())
	assert.True(t, b.IsCompleted())
	for v := range seen {
		require.True(t, seen[v].Load(), "value %d never came out", v)
	}
}
