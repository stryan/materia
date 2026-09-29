package facts

import (
	"errors"
	"fmt"
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
	var out []string
	var walk func(prefix Path, node any)
	walk = func(prefix Path, node any) {
		child := func(seg string) Path {
			return append(prefix[:len(prefix):len(prefix)], seg)
		}
		switch n := node.(type) {
		case map[string]any:
			for k, v := range n {
				walk(child(k), v)
			}
		case []any:
			for i, v := range n {
				walk(child(strconv.Itoa(i)), v)
			}
		default:
			out = append(out, prefix.String())
		}
	}
	walk(nil, f.rawFacts)
	sort.Strings(out)
	return out
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
				return nil, fmt.Errorf("%w: expected slice, got %v", ErrFactNotFound, index)
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
