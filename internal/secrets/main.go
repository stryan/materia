package secrets

import (
	"context"
)

type SecretsManager interface {
	All(context.Context) map[string]interface{}
	Lookup(context.Context, SecretFilter) map[string]interface{}
}

type SecretsConfig interface {
	SecretsType() string
	Validate() error
}

type SecretFilter struct {
	Hostname string
	Role     string
}
