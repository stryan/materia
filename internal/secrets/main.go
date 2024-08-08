package secrets

import (
	"context"
)

type SecretsManager interface {
	All(context.Context) map[string]interface{}
	Lookup(context.Context, SecretFilter) map[string]interface{}
}

type SecretFilter struct {
	Hostname string
	Role     string
}
