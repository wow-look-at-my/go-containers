package concurrentset

import (
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/go-containers/concurrentmap"
)

func TestNewIsEmpty(t *testing.T) {
	s := New[int]()
	assert.True(t, s.IsEmpty())
	assert.Zero(t, s.Len())
	assert.Empty(t, s.Values())
}

func TestAddReportsWhetherItWasNew(t *testing.T) {
	s := New[string]()
	assert.True(t, s.Add("a"), "expected the first add to report new")
	assert.False(t, s.Add("a"), "expected the second add of the same element to report not-new")
	assert.Equal(t, 1, s.Len())
}

func TestAddRange(t *testing.T) {
	s := New[int]()
	s.AddRange(1, 2, 3, 2, 1)
	assert.Equal(t, 3, s.Len(), "expected duplicates within the range to collapse")
	assert.True(t, s.Contains(1))
	assert.True(t, s.Contains(2))
	assert.True(t, s.Contains(3))
}

func TestRemove(t *testing.T) {
	s := New[int]()
	s.AddRange(1, 2, 3)
	s.Remove(2, 5) // 5 was never present
	assert.False(t, s.Contains(2))
	assert.Equal(t, 2, s.Len())

	// Removing everything else is a no-op, not an error.
	s.Remove(1, 3)
	assert.True(t, s.IsEmpty())
}

func TestContains(t *testing.T) {
	s := New[int]()
	assert.False(t, s.Contains(1))
	s.Add(1)
	assert.True(t, s.Contains(1))
}

func TestClear(t *testing.T) {
	s := New[int]()
	s.AddRange(1, 2, 3)
	s.Clear()
	assert.True(t, s.IsEmpty())
	assert.Zero(t, s.Len())
}

func TestValuesAndAll(t *testing.T) {
	s := New[int]()
	s.AddRange(1, 2, 3)

	values := s.Values()
	sort.Ints(values)
	assert.Equal(t, []int{1, 2, 3}, values)

	var walked []int
	for v := range s.All() {
		walked = append(walked, v)
	}
	sort.Ints(walked)
	assert.Equal(t, []int{1, 2, 3}, walked)
}

func TestAllStopsOnFalse(t *testing.T) {
	s := New[int]()
	s.AddRange(1, 2, 3)

	seen := 0
	for range s.All() {
		seen++
		break
	}
	assert.Equal(t, 1, seen, "expected the walk to stop as soon as yield returns false")
}

func TestString(t *testing.T) {
	s := New[int]()
	s.Add(1)
	assert.Equal(t, "[1]", s.String())
}

func TestNewForwardsOptions(t *testing.T) {
	s := New[int](concurrentmap.WithConcurrency(64))
	require.NotNil(t, s.m)
}

// TestConcurrentAddRemoveContains races many goroutines adding, removing, and
// reading the same small key space. The race detector is the real assertion
// here; the length checks just confirm the set never over- or under-counts.
func TestConcurrentAddRemoveContains(t *testing.T) {
	s := New[int]()
	const goroutines = 32
	const perGoroutine = 500
	const keySpace = 64

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				k := (seed + i) % keySpace
				switch i % 3 {
				case 0:
					s.Add(k)
				case 1:
					s.Contains(k)
				case 2:
					s.Remove(k)
				}
			}
		}(g)
	}
	wg.Wait()

	assert.LessOrEqual(t, s.Len(), keySpace, "the set can never hold more than the key space")
	assert.GreaterOrEqual(t, s.Len(), 0)
}
