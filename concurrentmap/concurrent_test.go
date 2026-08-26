package concurrentmap

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConcurrentAddOrUpdateCounters checks the sums, not the absence of a
// crash. Every increment must survive, or the update is not atomic.
func TestConcurrentAddOrUpdateCounters(t *testing.T) {
	const (
		goroutines = 16
		perGor     = 2000
		keys       = 10
	)
	m := New[int, int]()

	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perGor {
				m.AddOrUpdate(i%keys, 1, func(_ int, old int) int { return old + 1 })
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, keys, m.Len())
	want := goroutines * perGor / keys
	for k := range keys {
		assert.Equal(t, want, must(m.Load(k)), "key %d", k)
	}
}

// TestConcurrentLoadOrComputeRunsOnce proves the lock does what .NET's
// GetOrAdd cannot: one racing caller computes, and every caller sees its value.
func TestConcurrentLoadOrComputeRunsOnce(t *testing.T) {
	const goroutines = 64
	m := New[string, int]()

	var calls atomic.Int64
	var loadedCount atomic.Int64
	values := make([]int, goroutines)

	start := make(chan struct{})
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			v, loaded := m.LoadOrCompute("shared", func(string) int {
				calls.Add(1)
				return 42
			})
			values[g] = v
			if loaded {
				loadedCount.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	assert.EqualValues(t, 1, calls.Load(), "the compute function must run once")
	assert.EqualValues(t, goroutines-1, loadedCount.Load())
	for g := range goroutines {
		assert.Equal(t, 42, values[g], "goroutine %d", g)
	}
}

// TestConcurrentTryAddRunsOnce checks that exactly one caller wins an insert.
func TestConcurrentTryAddRunsOnce(t *testing.T) {
	const goroutines = 32
	m := New[string, int]()

	var winners atomic.Int64
	start := make(chan struct{})
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if m.TryAdd("shared", g) {
				winners.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	assert.EqualValues(t, 1, winners.Load())
	assert.Equal(t, 1, m.Len())
}

// TestConcurrentChurn runs stores, deletes, loads, Len and iteration together.
// The assertions are the invariants that hold once the writers stop.
func TestConcurrentChurn(t *testing.T) {
	const (
		keep   = 500 // keys the writers own; nobody deletes them
		doomed = 500 // keys the deleters own
		rounds = 20
	)
	m := New[int, int](WithConcurrency(16))
	for i := range doomed {
		m.Store(keep+i, i)
	}

	var mismatches atomic.Int64
	stop := make(chan struct{})
	var wg sync.WaitGroup

	for w := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				for i := w; i < keep; i += 4 {
					m.Store(i, i*2)
				}
			}
		}()
	}

	for d := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := d; i < doomed; i += 2 {
				m.Delete(keep + i)
			}
		}()
	}

	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				for i := range keep {
					if v, ok := m.Load(i); ok && v != i*2 {
						mismatches.Add(1)
					}
				}
			}
		}()
	}

	// The monitor uses its own group; wg is what tells the test to stop it.
	var monitor sync.WaitGroup
	monitor.Add(1)
	go func() {
		defer monitor.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			n := m.Len()
			if n < 0 || n > keep+doomed {
				mismatches.Add(1)
			}
			for k, v := range m.All() {
				if k < keep && v != k*2 {
					mismatches.Add(1)
				}
			}
		}
	}()

	wg.Wait()
	close(stop)
	monitor.Wait()

	assert.EqualValues(t, 0, mismatches.Load(), "a reader saw a value no writer wrote")
	assert.Equal(t, keep, m.Len())
	for i := range keep {
		assert.Equal(t, i*2, must(m.Load(i)), "key %d", i)
	}
	for i := range doomed {
		assert.False(t, m.Contains(keep+i), "key %d must be gone", keep+i)
	}
}

// TestConcurrentComputeIsAtomic builds a slice under Compute. A lost update
// shortens the slice.
func TestConcurrentComputeIsAtomic(t *testing.T) {
	const goroutines = 8
	const perGor = 500
	m := New[string, int]()

	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perGor {
				m.Compute("n", func(old int, _ bool) (int, bool) { return old + 1, false })
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, goroutines*perGor, must(m.Load("n")))
}

// TestConcurrentCompareAndSwap counts the winners of a fixed number of swaps.
func TestConcurrentCompareAndSwap(t *testing.T) {
	const goroutines = 16
	m := New[string, int]()
	m.Store("a", 0)

	var wins atomic.Int64
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				old := must(m.Load("a"))
				if old >= 1000 {
					return
				}
				if CompareAndSwap(m, "a", old, old+1) {
					wins.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 1000, must(m.Load("a")))
	assert.EqualValues(t, 1000, wins.Load())
}
