// Package event provides a generic, thread-safe event type with weak-referenced callbacks.
package event

import (
	"errors"
	"weak"
)

// EventArgs is a marker interface that constrains the argument type of an
// [Event] or [ResultEvent] to a struct embedding [Args]. This prevents using
// primitive types as event arguments, ensuring callers can always add fields
// later without breaking the signature.
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
//
// Callbacks produce only an error. When subscribers must also return a
// value, use [ResultEvent].
type Event[T EventArgs] struct {
	d dispatcher[func(T) error]
}

// Subscribe registers a callback with the event. The event stores a weak
// reference to cb; the caller must keep cb reachable to prevent garbage
// collection. Returns true if the callback was newly added, or false if
// it was already registered.
func (e *Event[T]) Subscribe(cb *func(T) error) bool {
	return e.d.subscribe(cb)
}

// Unsubscribe removes a previously registered callback.
func (e *Event[T]) Unsubscribe(cb *func(T) error) {
	e.d.unsubscribe(cb)
}

// Len returns the number of registered callbacks, including any that may
// have been garbage collected but not yet cleaned up.
func (e *Event[T]) Len() int {
	return e.d.len()
}

// Invoke calls every registered callback with arg. All live callbacks are
// called even if some return errors. Callbacks whose referents have been
// garbage collected are silently skipped and removed. The returned error,
// if non-nil, is the joined collection of all callback errors; a single
// subscriber's error is returned as it is, with nothing wrapped around it.
func (e *Event[T]) Invoke(arg T) error {
	if only, ok := e.d.takeOne(); ok {
		cb := only.Value()
		if cb == nil {
			e.d.remove(only)
			return nil
		}
		return (*cb)(arg)
	}

	snapshot := e.d.takeSnapshot()
	defer e.d.returnSnapshot(snapshot)

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

	e.d.removeAll(dead)
	return errors.Join(errs...)
}

// ResultEvent is [Event] whose callbacks also return a value of type R.
// Invoke collects every live callback's value, in snapshot order, and still
// joins errors the same way [Event.Invoke] does.
type ResultEvent[T EventArgs, R any] struct {
	d dispatcher[func(T) (R, error)]
}

// Subscribe registers a callback with the event. The event stores a weak
// reference to cb; the caller must keep cb reachable to prevent garbage
// collection. Returns true if the callback was newly added, or false if
// it was already registered.
func (e *ResultEvent[T, R]) Subscribe(cb *func(T) (R, error)) bool {
	return e.d.subscribe(cb)
}

// Unsubscribe removes a previously registered callback.
func (e *ResultEvent[T, R]) Unsubscribe(cb *func(T) (R, error)) {
	e.d.unsubscribe(cb)
}

// Len returns the number of registered callbacks, including any that may
// have been garbage collected but not yet cleaned up.
func (e *ResultEvent[T, R]) Len() int {
	return e.d.len()
}

// Invoke calls every registered callback with arg and returns each live
// callback's value. All live callbacks are called even if some return
// errors. A callback that returns a non-nil error still contributes its
// value. Callbacks whose referents have been garbage collected are
// silently skipped and removed. The returned error, if non-nil, is the
// joined collection of all callback errors; a single subscriber's error
// is returned as it is, with nothing wrapped around it.
func (e *ResultEvent[T, R]) Invoke(arg T) ([]R, error) {
	if only, ok := e.d.takeOne(); ok {
		cb := only.Value()
		if cb == nil {
			e.d.remove(only)
			return nil, nil
		}
		v, err := (*cb)(arg)
		return []R{v}, err
	}

	snapshot := e.d.takeSnapshot()
	defer e.d.returnSnapshot(snapshot)

	results := make([]R, 0, len(*snapshot))
	var errs []error
	var dead []weak.Pointer[func(T) (R, error)]

	for _, wp := range *snapshot {
		cb := wp.Value()
		if cb == nil {
			dead = append(dead, wp)
			continue
		}
		v, err := (*cb)(arg)
		results = append(results, v)
		if err != nil {
			errs = append(errs, err)
		}
	}

	e.d.removeAll(dead)
	return results, errors.Join(errs...)
}
