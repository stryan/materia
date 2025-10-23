package file

import (
	"errors"
	"fmt"

	"github.com/knadh/koanf/v2"
)

type Config struct {
	BaseDir       string   `toml:"base_dir"`
	GeneralVaults []string `toml:"vaults"`
}

func (c Config) Validate() error {
	if c.BaseDir == "" {
		return errors.New("need base directory for file attributes")
	}
	return nil
}

func NewConfig(k *koanf.Koanf) (*Config, error) {
	var c Config
	c.BaseDir = k.String("file.base_dir")
	if c.BaseDir == "" {
		c.BaseDir = "secrets"
	}
	c.GeneralVaults = k.Strings("file.vaults")

	return &c, nil
}

func (c *Config) Merge(other *Config) {
	if other.BaseDir != "" {
		c.BaseDir = other.BaseDir
	}
	if len(other.GeneralVaults) > 0 {
		c.GeneralVaults = append(c.GeneralVaults, other.GeneralVaults...)
	}
}

func (c Config) String() string {
	return fmt.Sprintf("Base Path: %v\nVaults: %v\n", c.BaseDir, c.GeneralVaults)
}

func (c Config) SourceType() string {
	return "file"
}
