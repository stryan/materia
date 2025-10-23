package age

import (
	"fmt"

	"github.com/knadh/koanf/v2"
)

type Config struct {
	IdentPath     string   `toml:"keyfile"`
	BaseDir       string   `toml:"base_dir"`
	GeneralVaults []string `toml:"vaults"`
}

func (c Config) Validate() error {
	if c.BaseDir == "" {
		return fmt.Errorf("empty base path for age")
	}
	if c.IdentPath == "" {
		return fmt.Errorf("empty identities location for age")
	}
	return nil
}

func (c Config) SourceType() string { return "age" }

func NewConfig(k *koanf.Koanf) (*Config, error) {
	var c Config
	c.IdentPath = k.String("age.keyfile")
	c.BaseDir = k.String("age.base_dir")
	if c.BaseDir == "" {
		c.BaseDir = "secrets"
	}
	c.GeneralVaults = k.Strings("age.vaults")
	if len(c.GeneralVaults) == 0 {
		c.GeneralVaults = []string{"vault.age", "attributes.age"}
	}
	return &c, nil
}

func (c *Config) Merge(other *Config) {
	if other.IdentPath != "" {
		c.IdentPath = other.IdentPath
	}
	if other.BaseDir != "" {
		c.BaseDir = other.BaseDir
	}
	if len(other.GeneralVaults) > 0 {
		c.GeneralVaults = append(c.GeneralVaults, other.GeneralVaults...)
	}
}

func (c Config) String() string {
	return fmt.Sprintf("Keyfile Path:%v\nBase Path: %v\nVaults: %v\n", c.IdentPath, c.BaseDir, c.GeneralVaults)
}
