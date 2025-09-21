package materia

import (
	"cmp"
	"slices"
)

func sortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	keys := make([]K, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	slices.Sort(keys)
	return keys
}
