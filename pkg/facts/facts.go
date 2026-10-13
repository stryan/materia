package facts

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
)

var ErrFactNotFound = errors.New("fact not found")

type Facts struct {
	typedFacts map[string]any
	rawFacts   map[string]any
}

// GetTypedFacts returns what a collector stored, as it was stored i.e. in the original struct
func GetTypedFacts[T any](f *Facts, namespace string) (T, error) {
	var og T
	fact, ok := f.typedFacts[namespace]
	if !ok {
		return og, fmt.Errorf("not a known namespace: %v", namespace)
	}
	t, ok := fact.(T)
	if !ok {
		return og, fmt.Errorf("incorrect fact type (%v) for namespace %v", fact, namespace)
	}
	return t, nil
}

func (f *Facts) Snapshot() map[string]any { return f.typedFacts }

func (f *Facts) Keys() []string {
	keys := getLeaves(nil, f.rawFacts)
	sort.Strings(keys)
	return keys
}

func getLeaves(prefix Path, node any) []string {
	var keys []string
	switch n := node.(type) {
	case map[string]any:
		for k, v := range n {
			keys = append(keys, getLeaves(slices.Concat(prefix, Path{k}), v)...)
		}
	case []any:
		for i, v := range n {
			keys = append(keys, getLeaves(slices.Concat(prefix, Path{strconv.Itoa(i)}), v)...)
		}
	default:
		keys = append(keys, prefix.String())
	}
	return keys
}

func (f *Facts) Lookup(path Path) (any, error) {
	var cur any = f.rawFacts
	for i, index := range path {
		switch node := cur.(type) {
		// we need to step deeper
		case map[string]any:
			nextNode, ok := node[index]
			if !ok {
				return nil, fmt.Errorf("%w: %v", ErrFactNotFound, index)
			}
			cur = nextNode
		// the current segment should be an index
		case []any:
			idx, err := strconv.Atoi(index)
			if err != nil {
				return nil, fmt.Errorf("%w: expected facts slice index, got %v", ErrFactNotFound, index)
			}
			if idx < 0 || idx > len(node) {
				return nil, fmt.Errorf("%w: facts index %v out of bounds for facts slice (%v)", ErrFactNotFound, idx, len(node))
			}
			cur = node[idx]
		// we dug too deep
		default:
			return nil, fmt.Errorf("%w: %s ", ErrFactNotFound, path[:i+1])
		}
	}
	return cur, nil
}

// LookupString is converts the key itself to a path for Lookup()
func (f *Facts) LookupString(key string) (any, error) {
	p, err := ParsePath(key)
	if err != nil {
		return nil, err
	}
	if _, ok := f.rawFacts[p[0]]; !ok {
		// not in an existing namespace, try defaulting to host
		// TODO add deprecation warning
		p = append(Path{"host"}, p...)
	}
	return f.Lookup(p)
}
