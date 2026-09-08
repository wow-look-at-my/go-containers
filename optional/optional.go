// Package optional provides Optional, a value that is either present or
// absent. It replaces a *T used only to mean "maybe", and it carries the
// comma-ok result of a lookup as a value.
package optional

import (
	"fmt"
	"iter"
)

// Optional holds a value of type T, or nothing. An unset Optional is empty and
// ready to use. The value sits inline, so it is as wide as a T plus a bool.
type Optional[T any] struct {
	value   T
	present bool
}

// Of returns an Optional holding value.
func Of[T any](value T) Optional[T] {
	return Optional[T]{value: value, present: true}
}

// Empty returns an empty Optional. It is the unset Optional, named.
func Empty[T any]() Optional[T] {
	return Optional[T]{}
}

// OfOk turns a comma-ok result into an Optional, so a lookup can be passed
// along as a value: optional.OfOk(m.Load(key)).
func OfOk[T any](value T, ok bool) Optional[T] {
	if !ok {
		return Optional[T]{}
	}
	return Optional[T]{value: value, present: true}
}

// OfPtr returns an Optional holding a copy of *p, or an empty Optional when p
// is nil.
func OfPtr[T any](p *T) Optional[T] {
	if p == nil {
		return Optional[T]{}
	}
	return Optional[T]{value: *p, present: true}
}

// Get returns the value and reports whether it is present. An absent value
// reads as the default T.
func (o Optional[T]) Get() (T, bool) {
	return o.value, o.present
}

// MustGet returns the value and panics when it is absent.
func (o Optional[T]) MustGet() T {
	if !o.present {
		panic("optional: MustGet on an empty Optional")
	}
	return o.value
}

// IsPresent reports whether a value is present.
func (o Optional[T]) IsPresent() bool {
	return o.present
}

// IsEmpty reports whether no value is present.
func (o Optional[T]) IsEmpty() bool {
	return !o.present
}

// IsZero reports whether the Optional is empty, which is what `omitzero` calls.
func (o Optional[T]) IsZero() bool {
	return !o.present
}

// OrElse returns the value, or fallback when no value is present.
func (o Optional[T]) OrElse(fallback T) T {
	if !o.present {
		return fallback
	}
	return o.value
}

// OrElseGet returns the value, or the result of fallback when no value is
// present. fallback runs only in the empty case.
func (o Optional[T]) OrElseGet(fallback func() T) T {
	if !o.present {
		return fallback()
	}
	return o.value
}

// OrZero returns the value, or the default T when no value is present.
func (o Optional[T]) OrZero() T {
	return o.value
}

// Ptr returns a pointer to a COPY of the value, or nil when it is absent: a
// caller cannot write through it into the Optional.
func (o Optional[T]) Ptr() *T {
	if !o.present {
		return nil
	}
	v := o.value
	return &v
}

// Set stores value, replacing whatever the Optional held.
func (o *Optional[T]) Set(value T) {
	o.value, o.present = value, true
}

// Clear removes the value, leaving an empty Optional. It drops the stored
// value, so a pointer the Optional held can be collected.
func (o *Optional[T]) Clear() {
	var zero T
	o.value, o.present = zero, false
}

// Filter returns the Optional unchanged when it holds a value that keep
// accepts, and an empty Optional otherwise.
func (o Optional[T]) Filter(keep func(T) bool) Optional[T] {
	if !o.present || !keep(o.value) {
		return Optional[T]{}
	}
	return o
}

// If calls do with the value when a value is present, and does nothing otherwise.
func (o Optional[T]) If(do func(T)) {
	if o.present {
		do(o.value)
	}
}

// All iterates the value when present and nothing when empty, so an Optional
// ranges like a collection.
func (o Optional[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		if o.present {
			yield(o.value)
		}
	}
}

// Values returns a slice holding the value, or an empty slice when no value is
// present.
func (o Optional[T]) Values() []T {
	if !o.present {
		return nil
	}
	return []T{o.value}
}

// String returns a human-readable representation of the Optional.
func (o Optional[T]) String() string {
	if !o.present {
		return "<empty>"
	}
	return fmt.Sprintf("%v", o.value)
}

// Map applies f to the value, and an empty Optional maps to an empty one with f
// never running. It is a function because a method cannot add a type parameter.
func Map[T, U any](o Optional[T], f func(T) U) Optional[U] {
	if !o.present {
		return Optional[U]{}
	}
	return Optional[U]{value: f(o.value), present: true}
}

// FlatMap applies f to the value and returns f's own Optional, so a chain of
// steps that can each come up empty does not nest.
func FlatMap[T, U any](o Optional[T], f func(T) Optional[U]) Optional[U] {
	if !o.present {
		return Optional[U]{}
	}
	return f(o.value)
}

// Equal reports whether a and b are both empty, or both hold the same value.
func Equal[T comparable](a, b Optional[T]) bool {
	if a.present != b.present {
		return false
	}
	return !a.present || a.value == b.value
}
