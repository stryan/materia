package mem

import (
	"context"

	"primamateria.systems/materia/internal/attributes"
)

type MemoryManager struct {
	secrets map[string]any
}

type MemoryConfig struct{}

func (m MemoryConfig) String() string {
	return ""
}

func (m MemoryConfig) Validate() error { return nil }

func (m MemoryConfig) SourceType() string { return "memory" }

func NewMemoryManager() *MemoryManager {
	secrets := make(map[string]any)
	return &MemoryManager{secrets}
}

func (m *MemoryManager) Lookup(_ context.Context, _ attributes.AttributesFilter) map[string]any {
	return m.secrets
}

func (m *MemoryManager) Add(key, value string) {
	m.secrets[key] = value
}
