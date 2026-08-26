// Package event provides a generic, thread-safe event type with weak-referenced callbacks.
package event

import (
	"errors"
	"weak"
)

// EventArgs constrains [Event]/[ResultEvent] to a struct embedding [Args].
type EventArgs interface {
	eventArgs()
}

// Args satisfies [EventArgs]; embed it in an argument struct as event.Args.
type Args struct{}

func (Args) eventArgs() {}

// Event dispatches to weak-ref callbacks over T; caller retains its *func(T) error. See [ResultEvent] for a return value.
type Event[T EventArgs] struct {
	d dispatcher[func(T) error]
}

// Subscribe holds a weak reference to cb, so the caller must keep it
// reachable; reports whether it was newly added.
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

// ResultEvent is [Event] whose callbacks also return R; Invoke collects every
// live value in snapshot order and joins errors the same way.
type ResultEvent[T EventArgs, R any] struct {
	d dispatcher[func(T) (R, error)]
}

// Subscribe holds a weak reference to cb, so the caller must keep it
// reachable; reports whether it was newly added.
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
