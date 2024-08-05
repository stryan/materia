package mem

import (
	"context"

	"github.com/nikolalohinski/gonja/v2/exec"
)

type MemoryManager struct {
	secrets map[string]interface{}
}

func NewMemoryManager() *MemoryManager {
	secrets := make(map[string]interface{})
	return &MemoryManager{secrets}
}

func (m *MemoryManager) All(_ context.Context) *exec.Context {
	return exec.NewContext(m.secrets)
}

func (m *MemoryManager) Add(key, value string) {
	m.secrets[key] = value
}
