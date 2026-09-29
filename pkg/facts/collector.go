package facts

import (
	"context"
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
