package orderedset

import "encoding/json"

// MarshalJSON implements the json.Marshaler interface. The set is serialized
// as a JSON array of its elements, in first-added order.
func (s OrderedSet[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Values())
}

// UnmarshalJSON implements the json.Unmarshaler interface. The set is read
// from a JSON array and takes that array's order, replacing whatever it held.
// A repeated element keeps the position of its first appearance.
func (s *OrderedSet[T]) UnmarshalJSON(data []byte) error {
	var elems []T
	if err := json.Unmarshal(data, &elems); err != nil {
		return err
	}
	s.index, s.order, s.dead = nil, nil, 0
	if len(elems) == 0 {
		return nil
	}
	s.index = make(map[T]int, len(elems))
	s.order = make([]T, 0, len(elems))
	s.AddRange(elems...)
	return nil
}
