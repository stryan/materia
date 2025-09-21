package sops

import (
	"fmt"

	"github.com/knadh/koanf/v2"
)

type Config struct {
	BaseDir       string   `toml:"BaseDir"`
	GeneralVaults []string `toml:"Vaults"`
}

func (c Config) Validate() error {
	if c.BaseDir == "" {
		return fmt.Errorf("empty base path for sops")
	}
	return nil
}

func (c Config) SecretsType() string { return "sops" }

func NewConfig(k *koanf.Koanf) (*Config, error) {
	var c Config
	c.BaseDir = k.String("sops.basedir")
	c.GeneralVaults = k.Strings("sops.vaults")
	if len(c.GeneralVaults) == 0 {
		c.GeneralVaults = []string{"vault.yml", "secrets.yml"}
	}
	return &c, nil
}

func (c *Config) Merge(other *Config) {
	if len(other.GeneralVaults) > 0 {
		c.GeneralVaults = append(c.GeneralVaults, other.GeneralVaults...)
	}
}

func (c Config) String() string {
	return fmt.Sprintf("Base Path: %v\nVaults: %v\n", c.BaseDir, c.GeneralVaults)
}
