// Package event provides a generic, thread-safe event type with weak-referenced callbacks.
package event

import (
	"errors"
	"sync"
	"weak"

	"github.com/wow-look-at-my/go-containers/set"
)

// EventArgs is a marker interface that constrains the argument type of an
// [Event] to a struct embedding [Args]. This prevents using primitive types
// as event arguments, ensuring callers can always add fields later without
// breaking the signature.
type EventArgs interface {
	eventArgs()
}

// Args is an embeddable base type that satisfies [EventArgs]. Embed it in
// your argument struct:
//
//	type ClickArgs struct {
//		event.Args
//		X, Y int
//	}
type Args struct{}

func (Args) eventArgs() {}

// Event is a thread-safe event dispatcher parameterized by a struct argument
// type T. T must embed [Args] to satisfy the [EventArgs] constraint.
// Registered callbacks are held as weak references, so callers must retain
// their own *func(T) error values to keep them alive.
// The zero value is ready to use.
type Event[T EventArgs] struct {
	mu        sync.RWMutex
	callbacks set.Set[weak.Pointer[func(T) error]]
	// snapshots recycles the buffer Invoke copies the callbacks into. The
	// buffer exists so the lock is released before any callback runs; keeping
	// it out of the collector's way costs one allocation per event, not one
	// per dispatch.
	snapshots sync.Pool
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
	snapshot := e.takeSnapshot()
	defer e.returnSnapshot(snapshot)

	var errs []error
	var dead []weak.Pointer[func(T) error]

	for _, wp := range *snapshot {
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

// takeSnapshot copies the registered callbacks into a recycled buffer. Invoke
// calls the callbacks with the lock released, so a callback may subscribe or
// unsubscribe; the copy is what makes that safe.
func (e *Event[T]) takeSnapshot() *[]weak.Pointer[func(T) error] {
	buf, _ := e.snapshots.Get().(*[]weak.Pointer[func(T) error])
	if buf == nil {
		buf = new([]weak.Pointer[func(T) error])
	}
	*buf = (*buf)[:0]

	e.mu.RLock()
	for wp := range e.callbacks.All() {
		*buf = append(*buf, wp)
	}
	e.mu.RUnlock()
	return buf
}

// returnSnapshot hands the buffer back, without the callbacks it held: a
// retained weak pointer would keep a dead subscriber's entry reachable.
func (e *Event[T]) returnSnapshot(buf *[]weak.Pointer[func(T) error]) {
	clear(*buf)
	*buf = (*buf)[:0]
	e.snapshots.Put(buf)
}
