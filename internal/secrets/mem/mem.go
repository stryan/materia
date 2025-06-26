package mem

import (
	"context"

	"primamateria.systems/materia/internal/secrets"
)

type MemoryManager struct {
	secrets map[string]any
}

type MemoryConfig struct{}

func (m MemoryConfig) String() string {
	return ""
}

func (m MemoryConfig) Validate() error { return nil }

func (m MemoryConfig) SecretsType() string { return "memory" }

func NewMemoryManager() *MemoryManager {
	secrets := make(map[string]any)
	return &MemoryManager{secrets}
}

func (m *MemoryManager) Lookup(_ context.Context, _ secrets.SecretFilter) map[string]any {
	return m.secrets
}

func (m *MemoryManager) Add(key, value string) {
	m.secrets[key] = value
}
