package optional

import "encoding/json"

var nullLiteral = []byte("null")

// MarshalJSON marshals a present value as itself and an empty Optional as
// null. Tag the field `json:",omitzero"` to leave it out instead.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.present {
		return nullLiteral, nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON decodes null to an empty Optional and anything else into T,
// replacing what the Optional held. An absent field never reaches here.
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
