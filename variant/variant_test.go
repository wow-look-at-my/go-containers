package variant

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZeroValueIsEmpty(t *testing.T) {
	var v Variant
	assert.True(t, v.IsEmpty())
	assert.False(t, v.IsPresent())
	assert.Nil(t, v.Value())
	assert.Nil(t, v.Type())
	assert.Equal(t, "<empty>", v.TypeName())
	assert.Equal(t, Empty(), v)

	_, ok := Get[int](v)
	assert.False(t, ok)
}

func TestGetAndIs(t *testing.T) {
	v := Of(42)
	n, ok := Get[int](v)
	require.True(t, ok)
	assert.Equal(t, 42, n)
	assert.True(t, Is[int](v))

	_, ok = Get[string](v)
	assert.False(t, ok, "the Variant holds an int, not a string")
	assert.False(t, Is[string](v))
	assert.Equal(t, "int", v.TypeName())
}

// An interface type parameter accepts any value that implements it, which is
// how a caller asks "does this hold an error" without naming the concrete
// type.
func TestGetThroughInterface(t *testing.T) {
	v := Of(errors.New("boom"))
	err, ok := Get[error](v)
	require.True(t, ok)
	assert.EqualError(t, err, "boom")
}

func TestMustGet(t *testing.T) {
	assert.Equal(t, "x", MustGet[string](Of("x")))
	assert.Panics(t, func() { MustGet[int](Of("x")) })
	assert.Panics(t, func() { MustGet[int](Empty()) })
}

// A held nil is present. Only Empty and Clear make a Variant empty.
func TestHeldNilIsPresent(t *testing.T) {
	v := Of(nil)
	assert.True(t, v.IsPresent())
	assert.Nil(t, v.Value())
	assert.Nil(t, v.Type())
	assert.Equal(t, "<nil>", v.TypeName())
}

func TestSetAndClear(t *testing.T) {
	var v Variant
	v.Set("a")
	assert.True(t, v.IsPresent())
	v.Set(1)
	assert.Equal(t, 1, v.Value(), "Set replaces what the Variant held")
	v.Clear()
	assert.True(t, v.IsEmpty())
	assert.Nil(t, v.Value(), "Clear drops the held value")
}

func TestString(t *testing.T) {
	assert.Equal(t, "42", Of(42).String())
	assert.Equal(t, "<empty>", Empty().String())
}

// The alternatives are variadic: the same Variant dispatches over as many
// handlers as the caller passes.
func TestSwitchOverManyAlternatives(t *testing.T) {
	seen := ""
	handlers := []any{
		func(n int) { seen = "int " + strconv.Itoa(n) },
		func(s string) { seen = "string " + s },
		func(b bool) { seen = "bool " + strconv.FormatBool(b) },
		func(f float64) { seen = "float " + strconv.FormatFloat(f, 'g', -1, 64) },
		func(e error) { seen = "error " + e.Error() },
	}

	for _, tc := range []struct {
		value any
		want  string
	}{
		{1, "int 1"},
		{"x", "string x"},
		{true, "bool true"},
		{2.5, "float 2.5"},
		{errors.New("boom"), "error boom"},
	} {
		seen = ""
		require.True(t, Of(tc.value).Switch(handlers...))
		assert.Equal(t, tc.want, seen)
	}
}

func TestSwitchReportsNoMatch(t *testing.T) {
	ran := Of([]byte("x")).Switch(func(int) {}, func(string) {})
	assert.False(t, ran, "no handler accepts a []byte")

	ran = Empty().Switch(func(int) { t.Fatal("an empty Variant runs nothing") })
	assert.False(t, ran)
}

// A func(any) accepts anything, so at the end of the list it is the default
// case.
func TestSwitchDefaultHandler(t *testing.T) {
	seen := ""
	require.True(t, Of(3.5).Switch(
		func(int) { seen = "int" },
		func(any) { seen = "default" },
	))
	assert.Equal(t, "default", seen)

	// The earliest accepting handler wins over the default behind it.
	seen = ""
	require.True(t, Of(7).Switch(
		func(int) { seen = "int" },
		func(any) { seen = "default" },
	))
	assert.Equal(t, "int", seen)
}

func TestSwitchHeldNilGoesToInterfaceHandler(t *testing.T) {
	seen := false
	assert.False(t, Of(nil).Switch(func(int) { t.Fatal("nil is not an int") }))
	require.True(t, Of(nil).Switch(func(any) { seen = true }))
	assert.True(t, seen)
}

func TestSwitchPanicsOnMalformedHandler(t *testing.T) {
	v := Of(1)
	assert.Panics(t, func() { v.Switch("not a function") })
	assert.Panics(t, func() { v.Switch(nil) })
	assert.Panics(t, func() { v.Switch(func(int) int { return 0 }) }, "a Switch handler returns nothing")
	assert.Panics(t, func() { v.Switch(func(int, int) {}) })
	assert.Panics(t, func() { v.Switch(func(...int) {}) })
}

func TestMatch(t *testing.T) {
	describe := func(v Variant) (string, bool) {
		return Match[string](v,
			func(n int) string { return "int " + strconv.Itoa(n) },
			func(s string) string { return "string " + s },
			func(any) string { return "other" },
		)
	}

	got, ok := describe(Of(3))
	require.True(t, ok)
	assert.Equal(t, "int 3", got)

	got, ok = describe(Of("x"))
	require.True(t, ok)
	assert.Equal(t, "string x", got)

	got, ok = describe(Of(2.5))
	require.True(t, ok)
	assert.Equal(t, "other", got)

	got, ok = describe(Empty())
	assert.False(t, ok)
	assert.Empty(t, got, "an empty Variant matches nothing and returns the empty result")
}

func TestMatchReportsNoMatch(t *testing.T) {
	got, ok := Match[int](Of("x"), func(int) int { return 1 })
	assert.False(t, ok)
	assert.Zero(t, got)
}

func TestMatchPanicsOnMalformedHandler(t *testing.T) {
	v := Of(1)
	assert.Panics(t, func() { Match[string](v, func(int) {}) }, "a Match handler returns a result")
	assert.Panics(t, func() { Match[string](v, func(int) int { return 0 }) }, "the result must be assignable to R")
	assert.Panics(t, func() { Match[string](v, "not a function") })
}

// A Match handler may return a type that is assignable to R rather than
// identical to it.
func TestMatchAssignableResult(t *testing.T) {
	got, ok := Match[any](Of(1), func(n int) int { return n + 1 })
	require.True(t, ok)
	assert.Equal(t, 2, got)
}
