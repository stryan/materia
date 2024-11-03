package mem

import (
	"context"

	"git.saintnet.tech/stryan/materia/internal/secrets"
)

type MemoryManager struct {
	secrets map[string]interface{}
}

type MemoryConfig struct{}

func (m MemoryConfig) Validate() error { return nil }

func (m MemoryConfig) SecretsType() string { return "memory" }

func NewMemoryManager() *MemoryManager {
	secrets := make(map[string]interface{})
	return &MemoryManager{secrets}
}

func (m *MemoryManager) All(_ context.Context) map[string]interface{} {
	return m.secrets
}

func (m *MemoryManager) Lookup(_ context.Context, _ secrets.SecretFilter) map[string]interface{} {
	return m.secrets
}

func (m *MemoryManager) Add(key, value string) {
	m.secrets[key] = value
}
