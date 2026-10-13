package facts

import (
	"errors"
	"fmt"
	"strings"
)

type Path []string

func (p Path) String() string {
	parts := make([]string, len(p))
	for i, s := range p {
		if strings.Contains(s, ".") {
			s = `"` + s + `"`
		}
		parts[i] = s
	}
	return strings.Join(parts, ".")
}

func ParsePath(key string) (Path, error) {
	if key == "" {
		return nil, errors.New("empty fact path")
	}
	var out Path
	var cur strings.Builder
	quoted := false
	for _, r := range key {
		switch {
		case r == '"':
			quoted = !quoted
		case r == '.' && !quoted:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if quoted {
		return nil, fmt.Errorf("missing a quote in fact path %v", key)
	}
	return append(out, cur.String()), nil
}
