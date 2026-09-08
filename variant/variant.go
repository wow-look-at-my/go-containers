// Package variant provides Variant2, Variant3 and Variant4: a value that
// holds exactly one of a fixed set of types. It is the tagged union Go has no
// syntax for, and it replaces a bare any plus a type switch over types the
// reader has to go and find.
package variant

import "fmt"

// variant is the tag and the payload that every arity shares. The slot is
// 1-based, so the zero value is an empty variant.
//
// The payload is an any, so a value larger than a pointer word is boxed on the
// way in. A per-arity struct with one typed field per alternative would skip
// that, at the price of three near-identical implementations and a value as
// wide as all of its alternatives at once.
type variant struct {
	slot int8
	val  any
}

// get returns the payload as T when the wanted slot is the one that holds the
// value. The assertion cannot fail: only the New and Set functions write a
// slot, and each writes the type that belongs to it.
func get[T any](v variant, want int8) (T, bool) {
	if v.slot != want {
		var zero T
		return zero, false
	}
	return v.val.(T), true
}

// Index returns the 1-based slot that holds the value: 1 for A, 2 for B, and
// so on. An empty variant returns 0.
func (v variant) Index() int {
	return int(v.slot)
}

// IsEmpty reports whether the variant holds no value at all.
func (v variant) IsEmpty() bool {
	return v.slot == 0
}

// Value returns the held value as an any, whichever slot holds it, or nil when
// the variant is empty. Use the typed accessors to learn which type it is.
func (v variant) Value() any {
	return v.val
}

// Clear empties the variant. It drops the payload, so a pointer the variant
// held can be collected.
func (v *variant) Clear() {
	v.slot, v.val = 0, nil
}

// String returns a human-readable representation of the variant.
func (v variant) String() string {
	if v.slot == 0 {
		return "<empty>"
	}
	return fmt.Sprintf("%v", v.val)
}

// ---------- two alternatives ----------

// Variant2 holds a value of type A or a value of type B. The zero value is
// empty.
type Variant2[A, B any] struct{ variant }

// New2A returns a Variant2 holding a. Both type arguments must be written out,
// because only A can be inferred from the argument.
func New2A[A, B any](a A) Variant2[A, B] { return Variant2[A, B]{variant{slot: 1, val: a}} }

// New2B returns a Variant2 holding b.
func New2B[A, B any](b B) Variant2[A, B] { return Variant2[A, B]{variant{slot: 2, val: b}} }

// A returns the value and true when the variant holds an A.
func (v Variant2[A, B]) A() (A, bool) { return get[A](v.variant, 1) }

// B returns the value and true when the variant holds a B.
func (v Variant2[A, B]) B() (B, bool) { return get[B](v.variant, 2) }

// SetA stores a, replacing whatever the variant held.
func (v *Variant2[A, B]) SetA(a A) { v.variant = variant{slot: 1, val: a} }

// SetB stores b, replacing whatever the variant held.
func (v *Variant2[A, B]) SetB(b B) { v.variant = variant{slot: 2, val: b} }

// Switch calls the handler for the slot that holds the value. An empty
// variant calls nothing, and so does a nil handler for the held slot, which is
// what lets a caller handle one alternative and ignore the rest.
func (v Variant2[A, B]) Switch(onA func(A), onB func(B)) {
	switch v.slot {
	case 1:
		call(onA, v.val)
	case 2:
		call(onB, v.val)
	}
}

// Match2 returns the result of the handler for the slot that holds the value.
// An empty variant returns the zero R and false. Every handler must be
// non-nil.
func Match2[A, B, R any](v Variant2[A, B], onA func(A) R, onB func(B) R) (R, bool) {
	switch v.slot {
	case 1:
		return onA(v.val.(A)), true
	case 2:
		return onB(v.val.(B)), true
	}
	var zero R
	return zero, false
}

// ---------- three alternatives ----------

// Variant3 holds a value of type A, B or C. The zero value is empty.
type Variant3[A, B, C any] struct{ variant }

// New3A returns a Variant3 holding a.
func New3A[A, B, C any](a A) Variant3[A, B, C] { return Variant3[A, B, C]{variant{slot: 1, val: a}} }

// New3B returns a Variant3 holding b.
func New3B[A, B, C any](b B) Variant3[A, B, C] { return Variant3[A, B, C]{variant{slot: 2, val: b}} }

// New3C returns a Variant3 holding c.
func New3C[A, B, C any](c C) Variant3[A, B, C] { return Variant3[A, B, C]{variant{slot: 3, val: c}} }

// A returns the value and true when the variant holds an A.
func (v Variant3[A, B, C]) A() (A, bool) { return get[A](v.variant, 1) }

// B returns the value and true when the variant holds a B.
func (v Variant3[A, B, C]) B() (B, bool) { return get[B](v.variant, 2) }

// C returns the value and true when the variant holds a C.
func (v Variant3[A, B, C]) C() (C, bool) { return get[C](v.variant, 3) }

// SetA stores a, replacing whatever the variant held.
func (v *Variant3[A, B, C]) SetA(a A) { v.variant = variant{slot: 1, val: a} }

// SetB stores b, replacing whatever the variant held.
func (v *Variant3[A, B, C]) SetB(b B) { v.variant = variant{slot: 2, val: b} }

// SetC stores c, replacing whatever the variant held.
func (v *Variant3[A, B, C]) SetC(c C) { v.variant = variant{slot: 3, val: c} }

// Switch calls the handler for the slot that holds the value, under the same
// rules as Variant2.Switch.
func (v Variant3[A, B, C]) Switch(onA func(A), onB func(B), onC func(C)) {
	switch v.slot {
	case 1:
		call(onA, v.val)
	case 2:
		call(onB, v.val)
	case 3:
		call(onC, v.val)
	}
}

// Match3 returns the result of the handler for the slot that holds the value,
// under the same rules as Match2.
func Match3[A, B, C, R any](v Variant3[A, B, C], onA func(A) R, onB func(B) R, onC func(C) R) (R, bool) {
	switch v.slot {
	case 1:
		return onA(v.val.(A)), true
	case 2:
		return onB(v.val.(B)), true
	case 3:
		return onC(v.val.(C)), true
	}
	var zero R
	return zero, false
}

// ---------- four alternatives ----------

// Variant4 holds a value of type A, B, C or D. The zero value is empty. Past
// four alternatives, a struct with named fields reads better than a position.
type Variant4[A, B, C, D any] struct{ variant }

// New4A returns a Variant4 holding a.
func New4A[A, B, C, D any](a A) Variant4[A, B, C, D] {
	return Variant4[A, B, C, D]{variant{slot: 1, val: a}}
}

// New4B returns a Variant4 holding b.
func New4B[A, B, C, D any](b B) Variant4[A, B, C, D] {
	return Variant4[A, B, C, D]{variant{slot: 2, val: b}}
}

// New4C returns a Variant4 holding c.
func New4C[A, B, C, D any](c C) Variant4[A, B, C, D] {
	return Variant4[A, B, C, D]{variant{slot: 3, val: c}}
}

// New4D returns a Variant4 holding d.
func New4D[A, B, C, D any](d D) Variant4[A, B, C, D] {
	return Variant4[A, B, C, D]{variant{slot: 4, val: d}}
}

// A returns the value and true when the variant holds an A.
func (v Variant4[A, B, C, D]) A() (A, bool) { return get[A](v.variant, 1) }

// B returns the value and true when the variant holds a B.
func (v Variant4[A, B, C, D]) B() (B, bool) { return get[B](v.variant, 2) }

// C returns the value and true when the variant holds a C.
func (v Variant4[A, B, C, D]) C() (C, bool) { return get[C](v.variant, 3) }

// D returns the value and true when the variant holds a D.
func (v Variant4[A, B, C, D]) D() (D, bool) { return get[D](v.variant, 4) }

// SetA stores a, replacing whatever the variant held.
func (v *Variant4[A, B, C, D]) SetA(a A) { v.variant = variant{slot: 1, val: a} }

// SetB stores b, replacing whatever the variant held.
func (v *Variant4[A, B, C, D]) SetB(b B) { v.variant = variant{slot: 2, val: b} }

// SetC stores c, replacing whatever the variant held.
func (v *Variant4[A, B, C, D]) SetC(c C) { v.variant = variant{slot: 3, val: c} }

// SetD stores d, replacing whatever the variant held.
func (v *Variant4[A, B, C, D]) SetD(d D) { v.variant = variant{slot: 4, val: d} }

// Switch calls the handler for the slot that holds the value, under the same
// rules as Variant2.Switch.
func (v Variant4[A, B, C, D]) Switch(onA func(A), onB func(B), onC func(C), onD func(D)) {
	switch v.slot {
	case 1:
		call(onA, v.val)
	case 2:
		call(onB, v.val)
	case 3:
		call(onC, v.val)
	case 4:
		call(onD, v.val)
	}
}

// Match4 returns the result of the handler for the slot that holds the value,
// under the same rules as Match2.
func Match4[A, B, C, D, R any](v Variant4[A, B, C, D], onA func(A) R, onB func(B) R, onC func(C) R, onD func(D) R) (R, bool) {
	switch v.slot {
	case 1:
		return onA(v.val.(A)), true
	case 2:
		return onB(v.val.(B)), true
	case 3:
		return onC(v.val.(C)), true
	case 4:
		return onD(v.val.(D)), true
	}
	var zero R
	return zero, false
}

// call runs handler on the payload, and does nothing when the caller left that
// slot's handler out.
func call[T any](handler func(T), val any) {
	if handler != nil {
		handler(val.(T))
	}
}
