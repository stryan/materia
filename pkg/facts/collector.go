package facts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

type Collector interface {
	Namespace() string
	Collect(context.Context, *Facts) (any, error)
}

func Collect(ctx context.Context, cs ...Collector) (*Facts, error) {
	f := &Facts{
		typedFacts: map[string]any{},
		rawFacts:   map[string]any{},
	}
	for _, v := range cs {
		newFacts, err := v.Collect(ctx, f)
		if err != nil {
			return nil, fmt.Errorf("failed to collect facts %v: %w", v.Namespace(), err)
		}
		serializedFacts, err := normalize(newFacts)
		if err != nil {
			return nil, fmt.Errorf("fact namespace %v contained unserializable data: %w", v.Namespace(), err)
		}
		f.typedFacts[v.Namespace()] = newFacts
		f.rawFacts[v.Namespace()] = serializedFacts
	}
	return f, nil
}

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
