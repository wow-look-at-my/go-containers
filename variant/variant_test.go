package variant

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZeroValueIsEmpty(t *testing.T) {
	var v Variant2[int, string]
	assert.True(t, v.IsEmpty())
	assert.Zero(t, v.Index())
	assert.Nil(t, v.Value())

	_, ok := v.A()
	assert.False(t, ok)
	_, ok = v.B()
	assert.False(t, ok)
}

func TestVariant2HoldsExactlyOneSlot(t *testing.T) {
	v := New2A[int, string](42)
	assert.Equal(t, 1, v.Index())

	a, ok := v.A()
	require.True(t, ok)
	assert.Equal(t, 42, a)

	_, ok = v.B()
	assert.False(t, ok, "the other slot is empty")

	v.SetB("hello")
	assert.Equal(t, 2, v.Index())
	_, ok = v.A()
	assert.False(t, ok, "SetB replaces what the variant held")
	b, ok := v.B()
	require.True(t, ok)
	assert.Equal(t, "hello", b)
}

// The two slots can hold the same type, and the tag still tells them apart.
func TestSameTypeInBothSlots(t *testing.T) {
	v := New2B[string, string]("x")
	assert.Equal(t, 2, v.Index())
	_, ok := v.A()
	assert.False(t, ok)
	b, ok := v.B()
	require.True(t, ok)
	assert.Equal(t, "x", b)
}

func TestClear(t *testing.T) {
	v := New2A[int, string](1)
	v.Clear()
	assert.True(t, v.IsEmpty())
	assert.Nil(t, v.Value(), "Clear drops the payload")
}

func TestSwitch(t *testing.T) {
	var seen string
	onA := func(n int) { seen = "a:" + strconv.Itoa(n) }
	onB := func(s string) { seen = "b:" + s }

	New2A[int, string](7).Switch(onA, onB)
	assert.Equal(t, "a:7", seen)

	New2B[int, string]("x").Switch(onA, onB)
	assert.Equal(t, "b:x", seen)

	var empty Variant2[int, string]
	seen = ""
	empty.Switch(onA, onB)
	assert.Empty(t, seen, "an empty variant calls nothing")
}

// A caller that cares about one alternative leaves the other handler out.
func TestSwitchIgnoresNilHandler(t *testing.T) {
	called := false
	assert.NotPanics(t, func() {
		New2B[int, string]("x").Switch(func(int) { called = true }, nil)
	})
	assert.False(t, called)
}

func TestMatch2(t *testing.T) {
	describe := func(v Variant2[int, string]) (string, bool) {
		return Match2(v,
			func(n int) string { return "int " + strconv.Itoa(n) },
			func(s string) string { return "string " + s },
		)
	}

	got, ok := describe(New2A[int, string](3))
	require.True(t, ok)
	assert.Equal(t, "int 3", got)

	got, ok = describe(New2B[int, string]("x"))
	require.True(t, ok)
	assert.Equal(t, "string x", got)

	var empty Variant2[int, string]
	got, ok = describe(empty)
	assert.False(t, ok)
	assert.Empty(t, got, "an empty variant matches nothing and returns the zero result")
}

func TestVariant3(t *testing.T) {
	v := New3C[int, string, bool](true)
	assert.Equal(t, 3, v.Index())
	c, ok := v.C()
	require.True(t, ok)
	assert.True(t, c)

	v.SetA(1)
	a, ok := v.A()
	require.True(t, ok)
	assert.Equal(t, 1, a)
	_, ok = v.C()
	assert.False(t, ok)

	v.SetB("two")
	b, ok := v.B()
	require.True(t, ok)
	assert.Equal(t, "two", b)

	got, ok := Match3(New3B[int, string, bool]("m"),
		func(int) string { return "a" },
		func(s string) string { return s },
		func(bool) string { return "c" },
	)
	require.True(t, ok)
	assert.Equal(t, "m", got)
}

func TestVariant4(t *testing.T) {
	v := New4D[int, string, bool, float64](1.5)
	assert.Equal(t, 4, v.Index())
	d, ok := v.D()
	require.True(t, ok)
	assert.InDelta(t, 1.5, d, 0)

	v.SetA(1)
	assert.Equal(t, 1, v.Index())
	v.SetB("b")
	assert.Equal(t, 2, v.Index())
	v.SetC(true)
	assert.Equal(t, 3, v.Index())
	c, ok := v.C()
	require.True(t, ok)
	assert.True(t, c)
	v.SetD(2.5)
	assert.Equal(t, 4, v.Index())

	slots := 0
	v.Switch(
		func(int) { slots++ },
		func(string) { slots++ },
		func(bool) { slots++ },
		func(float64) { slots++ },
	)
	assert.Equal(t, 1, slots, "exactly one handler runs")

	got, ok := Match4(New4C[int, string, bool, float64](true),
		func(int) string { return "a" },
		func(string) string { return "b" },
		func(bool) string { return "c" },
		func(float64) string { return "d" },
	)
	require.True(t, ok)
	assert.Equal(t, "c", got)
}

func TestValueAndString(t *testing.T) {
	v := New3B[int, string, bool]("payload")
	assert.Equal(t, "payload", v.Value())
	assert.Equal(t, "payload", v.String())

	var empty Variant3[int, string, bool]
	assert.Equal(t, "<empty>", empty.String())
}
