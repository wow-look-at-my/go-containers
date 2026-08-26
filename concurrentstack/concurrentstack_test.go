package concurrentstack

import (
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// drain pops everything the stack still holds, from the top down.
func drain[T any](s *Stack[T]) []T {
	var out []T
	for {
		v, ok := s.TryPop()
		if !ok {
			return out
		}
		out = append(out, v)
	}
}

func TestZeroValueIsReady(t *testing.T) {
	var s Stack[int]
	require.True(t, s.IsEmpty(), "expected the zero value to be empty")
	require.Equal(t, 0, s.Len(), "expected the zero value to hold nothing")

	_, ok := s.TryPop()
	assert.False(t, ok, "expected TryPop to report false on an empty stack")
	_, ok = s.TryPeek()
	assert.False(t, ok, "expected TryPeek to report false on an empty stack")

	s.Push(7)
	require.Equal(t, 1, s.Len(), "expected one value after Push")

	v, ok := s.TryPop()
	require.True(t, ok, "expected TryPop to report true")
	assert.Equal(t, 7, v, "expected the pushed value back")
}

func TestNew(t *testing.T) {
	s := New[string]()
	require.Equal(t, 0, s.Len(), "expected a new stack to be empty")
	assert.True(t, s.IsEmpty(), "expected a new stack to report empty")
	assert.Empty(t, s.Values(), "expected no values in a new stack")
}

func TestPushTryPopIsLIFO(t *testing.T) {
	s := New[int]()
	for i := 1; i <= 5; i++ {
		s.Push(i)
	}
	require.Equal(t, 5, s.Len(), "expected five values")
	assert.Equal(t, []int{5, 4, 3, 2, 1}, drain(s), "expected last-in-first-out order")
	assert.Equal(t, 0, s.Len(), "expected an empty stack after the drain")
}

func TestTryPopOnEmptyStack(t *testing.T) {
	s := New[int]()
	v, ok := s.TryPop()
	assert.False(t, ok, "expected TryPop to report false")
	assert.Equal(t, 0, v, "expected the zero value of T")

	s.Push(1)
	_, ok = s.TryPop()
	require.True(t, ok, "expected TryPop to report true")
	_, ok = s.TryPop()
	assert.False(t, ok, "expected TryPop to report false after the stack drained")
}

func TestTryPeek(t *testing.T) {
	s := New[string]()
	_, ok := s.TryPeek()
	require.False(t, ok, "expected TryPeek to report false on an empty stack")

	s.Push("a")
	s.Push("b")

	v, ok := s.TryPeek()
	require.True(t, ok, "expected TryPeek to report true")
	assert.Equal(t, "b", v, "expected the top value")
	assert.Equal(t, 2, s.Len(), "expected TryPeek to leave the value in place")

	again, ok := s.TryPeek()
	require.True(t, ok, "expected the second TryPeek to report true")
	assert.Equal(t, "b", again, "expected the same top value")
}

func TestPushRangeOrder(t *testing.T) {
	s := New[string]()
	s.PushRange("a", "b", "c")
	require.Equal(t, 3, s.Len(), "expected three values")

	// The last value pushed sits on top, exactly as three Push calls leave it.
	assert.Equal(t, []string{"c", "b", "a"}, drain(s), "expected c, b, a")

	one := New[int]()
	one.PushRange(42)
	v, ok := one.TryPop()
	require.True(t, ok, "expected TryPop to report true")
	assert.Equal(t, 42, v, "expected the single pushed value")
}

func TestPushRangeMatchesRepeatedPush(t *testing.T) {
	values := []int{1, 2, 3, 4, 5}

	byRange := New[int]()
	byRange.PushRange(values...)

	byPush := New[int]()
	for _, v := range values {
		byPush.Push(v)
	}

	assert.Equal(t, drain(byPush), drain(byRange), "expected PushRange to match repeated Push")
}

func TestPushRangeEmptyIsNoOp(t *testing.T) {
	s := New[int]()
	s.PushRange()
	assert.Equal(t, 0, s.Len(), "expected an empty PushRange to add nothing")

	var none []int
	s.PushRange(none...)
	assert.True(t, s.IsEmpty(), "expected the stack to stay empty")
}

func TestTryPopRange(t *testing.T) {
	s := New[int]()
	s.PushRange(1, 2, 3, 4, 5)

	buf := make([]int, 3)
	got := s.TryPopRange(buf)
	require.Equal(t, 3, got, "expected three values")
	assert.Equal(t, []int{5, 4, 3}, buf, "expected the top three, top first")
	assert.Equal(t, 2, s.Len(), "expected two values left")

	rest := make([]int, 10)
	got = s.TryPopRange(rest)
	require.Equal(t, 2, got, "expected the two remaining values")
	assert.Equal(t, []int{2, 1}, rest[:got], "expected the rest in order")
	assert.Equal(t, 0, s.Len(), "expected an empty stack")
}

func TestTryPopRangeEdgeCases(t *testing.T) {
	s := New[int]()

	buf := make([]int, 4)
	assert.Equal(t, 0, s.TryPopRange(buf), "expected zero from an empty stack")

	s.Push(1)
	assert.Equal(t, 0, s.TryPopRange(nil), "expected zero for a nil buffer")
	assert.Equal(t, 0, s.TryPopRange([]int{}), "expected zero for an empty buffer")
	assert.Equal(t, 1, s.Len(), "expected the value to stay on the stack")

	one := make([]int, 1)
	require.Equal(t, 1, s.TryPopRange(one), "expected one value")
	assert.Equal(t, 1, one[0], "expected the pushed value")
}

func TestLenIsEmptyAndClear(t *testing.T) {
	s := New[int]()
	assert.True(t, s.IsEmpty(), "expected a new stack to be empty")

	for i := range 10 {
		s.Push(i)
	}
	require.Equal(t, 10, s.Len(), "expected ten values")
	assert.False(t, s.IsEmpty(), "expected a filled stack not to report empty")

	s.Clear()
	assert.Equal(t, 0, s.Len(), "expected Clear to reset the length")
	assert.True(t, s.IsEmpty(), "expected an empty stack after Clear")
	_, ok := s.TryPop()
	assert.False(t, ok, "expected nothing to pop after Clear")

	s.Push(1)
	assert.Equal(t, 1, s.Len(), "expected the stack to work after Clear")

	empty := New[int]()
	empty.Clear()
	assert.Equal(t, 0, empty.Len(), "expected Clear on an empty stack to do nothing")
}

func TestValuesAndAll(t *testing.T) {
	s := New[int]()
	assert.Empty(t, s.Values(), "expected no values from an empty stack")

	s.PushRange(1, 2, 3)
	assert.Equal(t, []int{3, 2, 1}, s.Values(), "expected top-down order")
	assert.Equal(t, 3, s.Len(), "expected Values to leave the stack alone")

	assert.Equal(t, []int{3, 2, 1}, slices.Collect(s.All()), "expected All to match Values")

	var first []int
	for v := range s.All() {
		first = append(first, v)
		break
	}
	assert.Equal(t, []int{3}, first, "expected an early break to stop the walk")

	var seen int
	for range New[int]().All() {
		seen++
	}
	assert.Equal(t, 0, seen, "expected no iterations over an empty stack")
}

func TestTryAddTryTake(t *testing.T) {
	s := New[int]()
	assert.True(t, s.TryAdd(1), "expected TryAdd to report true")
	assert.True(t, s.TryAdd(2), "expected TryAdd to report true")
	require.Equal(t, 2, s.Len(), "expected two values")

	v, ok := s.TryTake()
	require.True(t, ok, "expected TryTake to report true")
	assert.Equal(t, 2, v, "expected the last value added")

	v, ok = s.TryTake()
	require.True(t, ok, "expected TryTake to report true")
	assert.Equal(t, 1, v, "expected the first value added")

	_, ok = s.TryTake()
	assert.False(t, ok, "expected TryTake to report false on an empty stack")
}

// ---------- concurrency ----------

const (
	producers        = 8
	valuesPerProduce = 2000
)

func TestConcurrentPushThenDrain(t *testing.T) {
	s := New[int]()

	var wg sync.WaitGroup
	for p := range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			base := p * valuesPerProduce
			for i := range valuesPerProduce {
				s.Push(base + i)
			}
		}()
	}
	wg.Wait()

	total := producers * valuesPerProduce
	require.Equal(t, total, s.Len(), "expected every push to be counted")

	got := drain(s)
	require.Len(t, got, total, "expected every value back exactly once")
	assert.Equal(t, 0, s.Len(), "expected Len to reach zero after the drain")

	slices.Sort(got)
	for i, v := range got {
		require.Equal(t, i, v, "expected value %d exactly once", i)
	}
}

func TestConcurrentProducersAndConsumers(t *testing.T) {
	s := New[int]()

	var done atomic.Bool
	taken := make([][]int, producers)

	var consumers sync.WaitGroup
	for c := range producers {
		consumers.Add(1)
		go func() {
			defer consumers.Done()
			for {
				if v, ok := s.TryPop(); ok {
					taken[c] = append(taken[c], v)
					continue
				}
				if done.Load() {
					return
				}
				runtime.Gosched()
			}
		}()
	}

	var writers sync.WaitGroup
	for p := range producers {
		writers.Add(1)
		go func() {
			defer writers.Done()
			base := p * valuesPerProduce
			for i := range valuesPerProduce {
				s.Push(base + i)
			}
		}()
	}
	writers.Wait()
	done.Store(true)
	consumers.Wait()

	// Taken plus what remains must equal exactly what was pushed.
	got := drain(s)
	for _, part := range taken {
		got = append(got, part...)
	}

	total := producers * valuesPerProduce
	require.Len(t, got, total, "expected every value exactly once")
	assert.Equal(t, 0, s.Len(), "expected Len to reach zero after the drain")

	slices.Sort(got)
	for i, v := range got {
		require.Equal(t, i, v, "expected value %d exactly once", i)
	}
}

func TestConcurrentStacksKeepGoroutineLIFO(t *testing.T) {
	// One goroutine's pushes must come back in its own reverse order.
	var wg sync.WaitGroup
	for g := range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := New[int]()
			want := make([]int, 0, 256)
			for i := range 256 {
				v := g*1000 + i
				s.Push(v)
				want = append(want, v)
			}
			slices.Reverse(want)
			assert.Equal(t, want, drain(s), "expected this goroutine's own LIFO order")
		}()
	}
	wg.Wait()
}

func TestConcurrentTryPopRangeTakesEachValueOnce(t *testing.T) {
	s := New[int]()
	total := producers * valuesPerProduce
	for i := range total {
		s.Push(i)
	}

	taken := make([][]int, producers)

	var wg sync.WaitGroup
	for c := range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]int, 32)
			for {
				got := s.TryPopRange(buf)
				if got == 0 {
					return
				}
				taken[c] = append(taken[c], buf[:got]...)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 0, s.Len(), "expected Len to reach zero after the drain")

	var all []int
	for _, part := range taken {
		all = append(all, part...)
	}
	require.Len(t, all, total, "expected every value in exactly one buffer")

	slices.Sort(all)
	for i, v := range all {
		require.Equal(t, i, v, "expected value %d exactly once", i)
	}
}

func TestConcurrentMixedOperationsKeepLenHonest(t *testing.T) {
	s := New[int]()

	var wg sync.WaitGroup
	for p := range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]int, 8)
			local := 0
			for i := range valuesPerProduce {
				switch i % 4 {
				case 0:
					s.Push(p)
				case 1:
					s.PushRange(p, p, p)
				case 2:
					s.TryPop()
					s.TryPeek()
				default:
					s.TryPopRange(buf)
				}
				local += s.Len()
			}
			lenSink.Add(int64(local))
		}()
	}
	wg.Wait()

	require.Len(t, s.Values(), s.Len(), "expected Len to match the chain once the goroutines stop")
	drain(s)
	assert.Equal(t, 0, s.Len(), "expected Len to reach zero after the drain")
}

// lenSink keeps the Len results of the mixed test alive without a data race.
var lenSink atomic.Int64
