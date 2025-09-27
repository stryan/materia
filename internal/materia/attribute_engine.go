package materia

import (
	"context"

	"primamateria.systems/materia/internal/attributes"
)

type AttributesEngine interface {
	Lookup(context.Context, attributes.AttributesFilter) map[string]any
}
