package materia

import (
	"fmt"

	"github.com/knadh/koanf/v2"
)

type RollbackConfig struct {
	Kind string `koanf:"kind" toml:"kind"`
}

func NewRollbackConfig(k *koanf.Koanf) (*RollbackConfig, error) {
	var c RollbackConfig
	c.Kind = k.String("rollback.kind")
	if c.Kind == "" {
		return nil, nil
	}
	return &c, nil
}

func (c *RollbackConfig) Validate() error {
	if c.Kind != "service" {
		return fmt.Errorf("invalid rollback type: %v", c.Kind)
	}
	return nil
}
