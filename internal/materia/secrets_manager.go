package materia

import (
	"context"

	"primamateria.systems/materia/internal/secrets"
)

type SecretsManager interface {
	Lookup(context.Context, secrets.SecretFilter) map[string]any
}
