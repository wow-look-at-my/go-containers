package optional

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZeroValueIsEmpty(t *testing.T) {
	var o Optional[int]
	assert.True(t, o.IsEmpty())
	assert.False(t, o.IsPresent())
	v, ok := o.Get()
	assert.False(t, ok)
	assert.Zero(t, v)
	assert.Equal(t, Empty[int](), o)
}

func TestOfAndGet(t *testing.T) {
	o := Of("hello")
	require.True(t, o.IsPresent())
	v, ok := o.Get()
	assert.True(t, ok)
	assert.Equal(t, "hello", v)
	assert.Equal(t, "hello", o.MustGet())
}

// A present zero value is present. That is the whole reason for the flag: a
// value cannot report its own absence.
func TestPresentZeroValue(t *testing.T) {
	o := Of(0)
	assert.True(t, o.IsPresent())
	assert.Equal(t, 0, o.OrElse(7))
}

func TestMustGetPanicsWhenEmpty(t *testing.T) {
	var o Optional[int]
	assert.Panics(t, func() { o.MustGet() })
}

func TestOfOk(t *testing.T) {
	m := map[string]int{"a": 1}

	present := OfOk(m["a"], true)
	assert.Equal(t, 1, present.OrZero())

	absent := OfOk(m["missing"], false)
	assert.True(t, absent.IsEmpty())
}

func TestOfPtr(t *testing.T) {
	n := 5
	assert.Equal(t, 5, OfPtr(&n).OrZero())
	assert.True(t, OfPtr[int](nil).IsEmpty())
}

// Ptr hands back a copy, so writing through it cannot reach into the Optional.
func TestPtrIsACopy(t *testing.T) {
	o := Of(1)
	p := o.Ptr()
	require.NotNil(t, p)
	*p = 99
	assert.Equal(t, 1, o.OrZero())

	assert.Nil(t, Empty[int]().Ptr())
}

func TestOrElseAndOrElseGet(t *testing.T) {
	assert.Equal(t, 1, Of(1).OrElse(2))
	assert.Equal(t, 2, Empty[int]().OrElse(2))

	called := false
	assert.Equal(t, 1, Of(1).OrElseGet(func() int { called = true; return 2 }))
	assert.False(t, called, "the fallback runs only when the value is absent")
	assert.Equal(t, 2, Empty[int]().OrElseGet(func() int { return 2 }))
}

func TestSetAndClear(t *testing.T) {
	var o Optional[string]
	o.Set("x")
	assert.True(t, o.IsPresent())
	o.Clear()
	assert.True(t, o.IsEmpty())
	assert.Empty(t, o.OrZero(), "Clear drops the stored value")
}

func TestFilter(t *testing.T) {
	even := func(n int) bool { return n%2 == 0 }
	assert.True(t, Of(2).Filter(even).IsPresent())
	assert.True(t, Of(3).Filter(even).IsEmpty())

	ran := false
	Empty[int]().Filter(func(int) bool { ran = true; return true })
	assert.False(t, ran, "an empty Optional never calls the predicate")
}

func TestIf(t *testing.T) {
	seen := 0
	Of(4).If(func(v int) { seen = v })
	assert.Equal(t, 4, seen)

	Empty[int]().If(func(int) { t.Fatal("If ran on an empty Optional") })
}

func TestAllAndValues(t *testing.T) {
	var got []int
	for v := range Of(3).All() {
		got = append(got, v)
	}
	assert.Equal(t, []int{3}, got)

	for range Empty[int]().All() {
		t.Fatal("an empty Optional yields nothing")
	}

	assert.Equal(t, []int{3}, Of(3).Values())
	assert.Empty(t, Empty[int]().Values())
}

func TestMapAndFlatMap(t *testing.T) {
	assert.Equal(t, "7", Map(Of(7), strconv.Itoa).OrZero())
	assert.True(t, Map(Empty[int](), strconv.Itoa).IsEmpty())

	half := func(n int) Optional[int] {
		if n%2 != 0 {
			return Empty[int]()
		}
		return Of(n / 2)
	}
	assert.Equal(t, 4, FlatMap(Of(8), half).OrZero())
	assert.True(t, FlatMap(Of(7), half).IsEmpty())
	assert.True(t, FlatMap(Empty[int](), half).IsEmpty())
}

func TestEqual(t *testing.T) {
	assert.True(t, Equal(Of(1), Of(1)))
	assert.False(t, Equal(Of(1), Of(2)))
	assert.True(t, Equal(Empty[int](), Empty[int]()))
	assert.False(t, Equal(Of(0), Empty[int]()), "a present zero is not absent")
}

func TestString(t *testing.T) {
	assert.Equal(t, "42", Of(42).String())
	assert.Equal(t, "<empty>", Empty[int]().String())
}

func TestJSONRoundTrip(t *testing.T) {
	type record struct {
		Name     string           `json:"name"`
		Nickname Optional[string] `json:"nickname"`
	}

	data, err := json.Marshal(record{Name: "ada", Nickname: Of("countess")})
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"ada","nickname":"countess"}`, string(data))

	data, err = json.Marshal(record{Name: "ada"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"ada","nickname":null}`, string(data))

	var back record
	require.NoError(t, json.Unmarshal([]byte(`{"name":"ada","nickname":"countess"}`), &back))
	assert.Equal(t, "countess", back.Nickname.OrZero())

	require.NoError(t, json.Unmarshal([]byte(`{"name":"ada","nickname":null}`), &back))
	assert.True(t, back.Nickname.IsEmpty(), "null unmarshals to empty, replacing what was there")

	var absent record
	require.NoError(t, json.Unmarshal([]byte(`{"name":"ada"}`), &absent))
	assert.True(t, absent.Nickname.IsEmpty(), "a missing field never reaches UnmarshalJSON")
}

// omitzero calls IsZero, which is how an empty Optional leaves its field out
// of the object rather than writing null.
func TestJSONOmitZero(t *testing.T) {
	type record struct {
		Nickname Optional[string] `json:",omitzero"`
	}

	data, err := json.Marshal(record{})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(data))

	data, err = json.Marshal(record{Nickname: Of("countess")})
	require.NoError(t, err)
	assert.JSONEq(t, `{"Nickname":"countess"}`, string(data))
}

func TestJSONUnmarshalError(t *testing.T) {
	var o Optional[int]
	assert.Error(t, o.UnmarshalJSON([]byte(`"not a number"`)))
	assert.True(t, o.IsEmpty(), "a failed decode leaves the Optional empty")
}
