// Package variant provides Variant, a value that holds a value of any type
// and remembers which type it is. It is the tagged union Go has no syntax for.
//
// The alternatives are variadic. There is no numbered family of types, because
// Go has no variadic type parameter and such a family caps the alternatives at
// whatever arity somebody bothered to write. The alternatives appear instead
// where a caller reads the value back, as the variadic handler list of Switch
// and Match.
package variant

import (
	"fmt"
	"reflect"
)

// Variant holds a value of any type, or nothing. An unset Variant is empty,
// ready to use. An any is what buys the variadic alternatives.
type Variant struct {
	val     any
	present bool
}

// Of returns a Variant holding value. A nil value is still held: only Empty and
// Clear make a Variant empty.
func Of(value any) Variant {
	return Variant{val: value, present: true}
}

// Empty returns an empty Variant. It is the unset Variant, named.
func Empty() Variant {
	return Variant{}
}

// Get returns the held value as T, and reports whether the Variant holds a T.
// It is a type assertion, so it costs no reflection.
func Get[T any](v Variant) (T, bool) {
	if !v.present {
		var zero T
		return zero, false
	}
	t, ok := v.val.(T)
	return t, ok
}

// Is reports whether the Variant holds a T.
func Is[T any](v Variant) bool {
	_, ok := Get[T](v)
	return ok
}

// MustGet returns the held value as T and panics when the Variant holds
// something else.
func MustGet[T any](v Variant) T {
	t, ok := Get[T](v)
	if !ok {
		panic(fmt.Sprintf("variant: holds %s, not %s", v.TypeName(), reflect.TypeFor[T]()))
	}
	return t
}

// Value returns the held value as an any, or nil when the Variant is empty.
func (v Variant) Value() any {
	return v.val
}

// IsPresent reports whether the Variant holds a value.
func (v Variant) IsPresent() bool {
	return v.present
}

// IsEmpty reports whether the Variant holds nothing.
func (v Variant) IsEmpty() bool {
	return !v.present
}

// Type returns the dynamic type of the held value, and nil for an empty Variant
// or a held nil.
func (v Variant) Type() reflect.Type {
	return reflect.TypeOf(v.val)
}

// TypeName returns the name of the held type, for a message a person reads.
func (v Variant) TypeName() string {
	if !v.present {
		return "<empty>"
	}
	if t := v.Type(); t != nil {
		return t.String()
	}
	return "<nil>"
}

// Set stores value, replacing whatever the Variant held.
func (v *Variant) Set(value any) {
	v.val, v.present = value, true
}

// Clear empties the Variant. It drops the held value, so a pointer the Variant
// held can be collected.
func (v *Variant) Clear() {
	v.val, v.present = nil, false
}

// String returns a human-readable representation of the Variant.
func (v Variant) String() string {
	if !v.present {
		return "<empty>"
	}
	return fmt.Sprintf("%v", v.val)
}

// Switch runs the earliest func(T) handler that accepts the held value, and
// reports whether any ran. A trailing func(any) is the default. Reflection
// picks it, so Get is the cheap path for a type the caller knows.
func (v Variant) Switch(handlers ...any) bool {
	if !v.present {
		return false
	}
	for i, h := range handlers {
		fn := reflect.ValueOf(h)
		t := handlerType(fn, i, 0)
		arg, ok := argFor(v.val, t.In(0))
		if !ok {
			continue
		}
		fn.Call([]reflect.Value{arg})
		return true
	}
	return false
}

// Match returns the result of the handler that accepts the held value, and
// reports whether any handler ran. Each handler is a func(T) R, under the
// matching rules of Switch.
//
// It is a function rather than a method because a Go method cannot introduce
// the result type parameter.
func Match[R any](v Variant, handlers ...any) (R, bool) {
	var zero R
	if !v.present {
		return zero, false
	}
	want := reflect.TypeFor[R]()
	for i, h := range handlers {
		fn := reflect.ValueOf(h)
		t := handlerType(fn, i, 1)
		if !t.Out(0).AssignableTo(want) {
			panic(fmt.Sprintf("variant: Match handler at index %d returns %s, want %s", i, t.Out(0), want))
		}
		arg, ok := argFor(v.val, t.In(0))
		if !ok {
			continue
		}
		out := fn.Call([]reflect.Value{arg})[0]
		return out.Interface().(R), true
	}
	return zero, false
}

// handlerType checks the shape of a handler and returns its type. results is
// how many values the handler must return. A handler of another shape panics:
// that is a mistake in the call, not in the data.
func handlerType(fn reflect.Value, index, results int) reflect.Type {
	if !fn.IsValid() || fn.Kind() != reflect.Func {
		panic(fmt.Sprintf("variant: handler at index %d is %s, want a function", index, kindOf(fn)))
	}
	t := fn.Type()
	if t.NumIn() != 1 || t.IsVariadic() || t.NumOut() != results {
		panic(fmt.Sprintf("variant: handler at index %d is %s, want a function of a parameter returning %d result(s)", index, t, results))
	}
	return t
}

// kindOf describes a handler that is not a function, including a nil handler.
func kindOf(fn reflect.Value) string {
	if !fn.IsValid() {
		return "nil"
	}
	return fn.Type().String()
}

// argFor returns the held value as the handler's parameter type, and reports
// whether that type accepts it. A held nil goes only to a handler that takes
// an interface.
func argFor(val any, param reflect.Type) (reflect.Value, bool) {
	if val == nil {
		if param.Kind() != reflect.Interface {
			return reflect.Value{}, false
		}
		return reflect.Zero(param), true
	}
	rv := reflect.ValueOf(val)
	if !rv.Type().AssignableTo(param) {
		return reflect.Value{}, false
	}
	return rv, true
}
