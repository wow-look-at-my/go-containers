package optional

import "encoding/json"

// nullLiteral is what an empty Optional marshals to, and what unmarshals back
// to an empty Optional.
var nullLiteral = []byte("null")

// MarshalJSON implements the json.Marshaler interface. A present value
// marshals as itself; an empty Optional marshals as null. Tag the field
// `json:",omitzero"` to leave it out of the object instead.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.present {
		return nullLiteral, nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON implements the json.Unmarshaler interface. JSON null becomes
// an empty Optional; anything else is decoded into T. Either way it replaces
// whatever the Optional held.
//
// A field that is absent from the object never reaches this method, so it
// stays empty.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		o.Clear()
		return nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.value, o.present = v, true
	return nil
}
