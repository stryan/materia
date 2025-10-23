package sops

import (
	"fmt"

	"github.com/knadh/koanf/v2"
)

type Config struct {
	BaseDir       string   `toml:"base_dir"`
	Suffix        string   `toml:"suffix"`
	GeneralVaults []string `toml:"vaults"`
}

func (c Config) Validate() error {
	if c.BaseDir == "" {
		return fmt.Errorf("empty base path for sops")
	}
	return nil
}

func (c Config) SourceType() string { return "sops" }

func NewConfig(k *koanf.Koanf) (*Config, error) {
	var c Config
	c.BaseDir = k.String("sops.base_dir")
	if c.BaseDir == "" {
		c.BaseDir = "secrets"
	}
	c.GeneralVaults = k.Strings("sops.vaults")
	c.Suffix = k.String("sops.suffix")
	if len(c.GeneralVaults) == 0 {
		c.GeneralVaults = []string{"vault.yml", "attributes.yml"}
		if c.Suffix != "" {
			c.GeneralVaults = append(c.GeneralVaults, fmt.Sprintf("vault.%v.yml", c.Suffix))
			c.GeneralVaults = append(c.GeneralVaults, fmt.Sprintf("attributes.%v.yml", c.Suffix))
		}
	}
	return &c, nil
}

func (c *Config) Merge(other *Config) {
	if len(other.GeneralVaults) > 0 {
		c.GeneralVaults = append(c.GeneralVaults, other.GeneralVaults...)
	}
}

func (c Config) String() string {
	return fmt.Sprintf("Base Path: %v\nSuffix: %v\nVaults: %v\n", c.BaseDir, c.Suffix, c.GeneralVaults)
}
