package mem

import (
	"context"

	"primamateria.systems/materia/internal/attributes"
)

type MemoryEngine struct {
	secrets map[string]any
}

type MemoryConfig struct{}

func (m MemoryConfig) String() string {
	return ""
}

func (m MemoryConfig) Validate() error { return nil }

func (m MemoryConfig) SourceType() string { return "memory" }

func NewMemoryEngine() *MemoryEngine {
	secrets := make(map[string]any)
	return &MemoryEngine{secrets}
}

func (m *MemoryEngine) Lookup(_ context.Context, _ attributes.AttributesFilter) map[string]any {
	return m.secrets
}

func (m *MemoryEngine) Add(key, value string) {
	m.secrets[key] = value
}
