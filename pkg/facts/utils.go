package facts

import (
	"bytes"
	"encoding/json"
)

// normalize takes a Facts struct as an any, serializes it through json, then returns the de-serialized map
func normalize(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var tree map[string]any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	return tree, nil
}
