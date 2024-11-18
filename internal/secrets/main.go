package secrets

import (
	"context"
)

type SecretsManager interface {
	Lookup(context.Context, SecretFilter) map[string]interface{}
}

type SecretsConfig interface {
	SecretsType() string
	Validate() error
}

type SecretFilter struct {
	Hostname  string
	Role      string
	Component string
}

type SecretsVault struct {
	Globals    map[string]interface{}
	Components map[string]map[string]interface{}
	Hosts      map[string]map[string]interface{}
	Roles      map[string]map[string]interface{}
}
