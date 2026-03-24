package set

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalJSON_Empty(t *testing.T) {
	s := New[int]()
	data, err := json.Marshal(s)
	require.NoError(t, err)
	assert.JSONEq(t, `[]`, string(data))
}

func TestMarshalJSON_ZeroValue(t *testing.T) {
	var s Set[int]
	data, err := json.Marshal(s)
	require.NoError(t, err)
	assert.JSONEq(t, `[]`, string(data))
}

func TestMarshalJSON_WithElements(t *testing.T) {
	s := Of(42)
	data, err := json.Marshal(s)
	require.NoError(t, err)
	assert.JSONEq(t, `[42]`, string(data))
}

func TestUnmarshalJSON_Basic(t *testing.T) {
	var s Set[int]
	err := json.Unmarshal([]byte(`[1,2,3]`), &s)
	require.NoError(t, err)
	assert.Equal(t, 3, s.Len())
	assert.True(t, s.ContainsAll(1, 2, 3))
}

func TestUnmarshalJSON_Null(t *testing.T) {
	var s Set[int]
	err := json.Unmarshal([]byte(`null`), &s)
	require.NoError(t, err)
	assert.True(t, s.IsEmpty())
}

func TestUnmarshalJSON_EmptyArray(t *testing.T) {
	var s Set[int]
	err := json.Unmarshal([]byte(`[]`), &s)
	require.NoError(t, err)
	assert.True(t, s.IsEmpty())
}

func TestUnmarshalJSON_ReplacesExisting(t *testing.T) {
	s := Of(10, 20, 30)
	err := json.Unmarshal([]byte(`[4,5]`), &s)
	require.NoError(t, err)
	assert.Equal(t, 2, s.Len())
	assert.True(t, s.ContainsAll(4, 5))
	assert.False(t, s.Contains(10))
}

func TestUnmarshalJSON_InvalidJSON(t *testing.T) {
	var s Set[int]
	err := json.Unmarshal([]byte(`not json`), &s)
	assert.Error(t, err)
}

func TestUnmarshalJSON_WrongType(t *testing.T) {
	var s Set[int]
	err := json.Unmarshal([]byte(`["a","b"]`), &s)
	assert.Error(t, err)
}

func TestJSON_RoundTrip_Ints(t *testing.T) {
	original := Of(1, 2, 3, 4, 5)
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored Set[int]
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)
	assert.True(t, original.Equal(restored))
}

func TestJSON_RoundTrip_Strings(t *testing.T) {
	original := Of("alpha", "beta", "gamma")
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored Set[string]
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)
	assert.True(t, original.Equal(restored))
}

func TestJSON_InStruct(t *testing.T) {
	type Config struct {
		Tags Set[string] `json:"tags"`
	}

	original := Config{Tags: Of("go", "containers")}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored Config
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)
	assert.True(t, original.Tags.Equal(restored.Tags))
}

func TestJSON_DuplicatesInInput(t *testing.T) {
	var s Set[int]
	err := json.Unmarshal([]byte(`[1,1,2,2,3]`), &s)
	require.NoError(t, err)
	assert.Equal(t, 3, s.Len())
	assert.True(t, s.ContainsAll(1, 2, 3))
}
