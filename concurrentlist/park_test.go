package concurrentlist

import (
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reserve claims the next slot the way Append does and leaves it unpublished.
func reserve[T any](l *List[T]) *slot[T] {
	s := l.tailSegment()
	pos := s.tail.Add(1) - 1
	return &s.slots[pos]
}

// A take that claims a slot whose producer has not published parks, and the
// publish wakes it. synctest.Wait returns only once the taker is durably
// blocked, so a taker that spun or yielded would hang the test here.
func TestTakeParksUntilTheProducerPublishes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := New[int]()
		pending := reserve(l)

		var got int
		var ok bool
		done := make(chan struct{})
		go func() {
			defer close(done)
			got, ok = l.TryTake()
		}()

		synctest.Wait()
		select {
		case <-done:
			t.Fatal("TryTake returned before the producer published")
		default:
		}
		assert.Equal(t, int32(1), l.parked.Load())

		l.publish(pending, 42)
		<-done
		require.True(t, ok)
		assert.Equal(t, 42, got)
		assert.Equal(t, int32(0), l.parked.Load())
	})
}

// Peekers and a range take parked on the same unpublished slot all wake on its publish.
func TestEveryParkedReaderWakesOnPublish(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := New[int]()
		pending := reserve(l)

		const peekers = 3
		peeked := make(chan int, peekers)
		for range peekers {
			go func() {
				v, ok := l.TryPeek()
				if ok {
					peeked <- v
				}
			}()
		}
		synctest.Wait()
		assert.Equal(t, int32(peekers), l.parked.Load())

		l.publish(pending, 9)
		for range peekers {
			assert.Equal(t, 9, <-peeked)
		}

		second := reserve(l)
		buf := make([]int, 2)
		taken := make(chan int)
		go func() { taken <- l.TryTakeRange(buf) }()
		synctest.Wait()
		assert.Equal(t, int32(1), l.parked.Load())

		l.publish(second, 10)
		require.Equal(t, 2, <-taken)
		assert.Equal(t, []int{9, 10}, buf)
	})
}

// A publish on one slot wakes a reader parked on another, which parks again
// until its own slot is published.
func TestAnotherSlotsPublishLeavesTheReaderParked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := New[int]()
		first := reserve(l)
		second := reserve(l)

		results := make(chan int, 2)
		go func() {
			v, _ := l.TryTake()
			results <- v
		}()
		synctest.Wait()

		buf := make([]int, 1)
		go func() {
			l.TryTakeRange(buf)
			results <- buf[0]
		}()
		synctest.Wait()
		assert.Equal(t, int32(2), l.parked.Load())

		l.publish(second, 2)
		synctest.Wait()
		assert.Equal(t, int32(1), l.parked.Load())
		assert.Equal(t, 2, <-results)

		l.publish(first, 1)
		assert.Equal(t, 1, <-results)
	})
}
