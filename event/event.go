// Package event provides a generic, thread-safe event type with weak-referenced callbacks.
package event

import (
	"errors"
	"sync"
	"weak"

	"github.com/wow-look-at-my/go-containers/set"
)

// Event is a thread-safe event dispatcher parameterized by the callback
// argument type T. Registered callbacks are held as weak references, so
// callers must retain their own *func(T) error values to keep them alive.
// The zero value is ready to use.
type Event[T any] struct {
	mu        sync.RWMutex
	callbacks set.Set[weak.Pointer[func(T) error]]
}

// Subscribe registers a callback with the event. The event stores a weak
// reference to cb; the caller must keep cb reachable to prevent garbage
// collection. Returns true if the callback was newly added, or false if
// it was already registered.
func (e *Event[T]) Subscribe(cb *func(T) error) bool {
	wp := weak.Make(cb)
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.callbacks.Add(wp)
}

// Unsubscribe removes a previously registered callback.
func (e *Event[T]) Unsubscribe(cb *func(T) error) {
	wp := weak.Make(cb)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.callbacks.Remove(wp)
}

// Len returns the number of registered callbacks, including any that may
// have been garbage collected but not yet cleaned up.
func (e *Event[T]) Len() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.callbacks.Len()
}

// Invoke calls every registered callback with arg. All live callbacks are
// called even if some return errors. Callbacks whose referents have been
// garbage collected are silently skipped and removed. The returned error,
// if non-nil, is the joined collection of all callback errors.
func (e *Event[T]) Invoke(arg T) error {
	e.mu.RLock()
	snapshot := e.callbacks.Values()
	e.mu.RUnlock()

	var errs []error
	var dead []weak.Pointer[func(T) error]

	for _, wp := range snapshot {
		cb := wp.Value()
		if cb == nil {
			dead = append(dead, wp)
			continue
		}
		if err := (*cb)(arg); err != nil {
			errs = append(errs, err)
		}
	}

	if len(dead) > 0 {
		e.mu.Lock()
		for _, d := range dead {
			e.callbacks.Remove(d)
		}
		e.mu.Unlock()
	}

	return errors.Join(errs...)
}
