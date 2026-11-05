package materia

import (
	"fmt"

	"github.com/knadh/koanf/v2"
)

type RollbackConfig struct {
	Kind string `koanf:"kind" toml:"kind"`
}

func NewRollbackConfig(k *koanf.Koanf) (RollbackConfig, error) {
	c := DefaultRollbackConfig()
	if err := k.UnmarshalWithConf("rollback", &c, koanf.UnmarshalConf{}); err != nil {
		return RollbackConfig{}, fmt.Errorf("unable to create rollback config: %w", err)
	}
	return c, nil
}

func (c RollbackConfig) Validate() error {
	if c.Kind != "service" && c.Kind != "" {
		return fmt.Errorf("invalid rollback type: %v", c.Kind)
	}
	return nil
}

func (c RollbackConfig) String() string {
	if c.Kind != "" {
		return fmt.Sprintf("Rollback mode: %v\n", c.Kind)
	}
	return "Rollback mode: None\n"
}

func DefaultRollbackConfig() RollbackConfig {
	return RollbackConfig{}
}
