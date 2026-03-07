package event

import (
	"errors"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZeroValueInvoke(t *testing.T) {
	var e Event[int]
	err := e.Invoke(42)
	assert.NoError(t, err, "invoking zero-value event should not error")
}

func TestSubscribeAndInvoke(t *testing.T) {
	var e Event[string]
	var called bool
	cb := func(s string) error {
		called = true
		assert.Equal(t, "hello", s)
		return nil
	}
	require.True(t, e.Subscribe(&cb), "expected Subscribe to return true for new callback")
	require.Equal(t, 1, e.Len())

	err := e.Invoke("hello")
	assert.NoError(t, err)
	assert.True(t, called, "expected callback to be called")
}

func TestSubscribeDuplicate(t *testing.T) {
	var e Event[int]
	cb := func(int) error { return nil }
	require.True(t, e.Subscribe(&cb), "first subscribe should succeed")
	assert.False(t, e.Subscribe(&cb), "duplicate subscribe should return false")
	assert.Equal(t, 1, e.Len(), "duplicate should not increase count")
}

func TestUnsubscribe(t *testing.T) {
	var e Event[int]
	callCount := 0
	cb := func(int) error { callCount++; return nil }
	e.Subscribe(&cb)
	e.Unsubscribe(&cb)
	assert.Equal(t, 0, e.Len())

	_ = e.Invoke(1)
	assert.Equal(t, 0, callCount, "unsubscribed callback should not be called")
}

func TestMultipleCallbacks(t *testing.T) {
	var e Event[int]
	var order []string

	cb1 := func(int) error { order = append(order, "a"); return nil }
	cb2 := func(int) error { order = append(order, "b"); return nil }
	cb3 := func(int) error { order = append(order, "c"); return nil }
	e.Subscribe(&cb1)
	e.Subscribe(&cb2)
	e.Subscribe(&cb3)
	require.Equal(t, 3, e.Len())

	err := e.Invoke(0)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(order), "expected all 3 callbacks to fire")
}

func TestErrorCollection(t *testing.T) {
	var e Event[int]

	errA := errors.New("error a")
	errB := errors.New("error b")

	cb1 := func(int) error { return errA }
	cb2 := func(int) error { return nil }
	cb3 := func(int) error { return errB }

	e.Subscribe(&cb1)
	e.Subscribe(&cb2)
	e.Subscribe(&cb3)

	err := e.Invoke(0)
	require.Error(t, err, "expected joined error")
	assert.ErrorIs(t, err, errA, "expected errA in joined error")
	assert.ErrorIs(t, err, errB, "expected errB in joined error")
}

func TestAllCallbacksRunDespiteErrors(t *testing.T) {
	var e Event[int]
	callCount := 0

	cb1 := func(int) error { callCount++; return errors.New("fail1") }
	cb2 := func(int) error { callCount++; return errors.New("fail2") }
	cb3 := func(int) error { callCount++; return nil }

	e.Subscribe(&cb1)
	e.Subscribe(&cb2)
	e.Subscribe(&cb3)

	_ = e.Invoke(0)
	assert.Equal(t, 3, callCount, "all callbacks should run even when some error")
}

func TestWeakReferenceCleanup(t *testing.T) {
	var e Event[int]
	called := false

	// Subscribe a callback via a pointer that we'll drop.
	cb := func(int) error { called = true; return nil }
	e.Subscribe(&cb)
	require.Equal(t, 1, e.Len())

	// Drop the only strong reference and force GC.
	cb = nil
	runtime.GC()

	err := e.Invoke(0)
	assert.NoError(t, err)
	assert.False(t, called, "GC'd callback should not be called")
	assert.Equal(t, 0, e.Len(), "dead callback should be cleaned up after Invoke")
}

func TestConcurrentSubscribeInvoke(t *testing.T) {
	var e Event[int]
	var wg sync.WaitGroup

	// Keep callbacks alive for the duration of the test.
	callbacks := make([]func(int) error, 100)
	for i := range callbacks {
		callbacks[i] = func(int) error { return nil }
	}

	// Concurrent subscribes.
	for i := range callbacks {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			e.Subscribe(&callbacks[idx])
		}(i)
	}
	wg.Wait()

	// Concurrent invokes.
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = e.Invoke(1)
		}()
	}
	wg.Wait()
}

func TestDistinctCallbackPointers(t *testing.T) {
	var e Event[int]
	// Two distinct pointers to identical functions should both register.
	cb1 := func(int) error { return nil }
	cb2 := func(int) error { return nil }
	assert.True(t, e.Subscribe(&cb1))
	assert.True(t, e.Subscribe(&cb2))
	assert.Equal(t, 2, e.Len())
}
