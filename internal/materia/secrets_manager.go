package materia

import (
	"context"

	"git.saintnet.tech/stryan/materia/internal/secrets"
)

type SecretsManager interface {
	Lookup(context.Context, secrets.SecretFilter) map[string]any
}
