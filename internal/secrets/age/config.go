package age

import (
	"errors"

	"github.com/knadh/koanf/v2"
)

type Config struct {
	IdentPath string
	RepoPath  string
}

func (c Config) Validate() error {
	if c.RepoPath == "" {
		return errors.New("invalid repo path for age")
	}
	if c.IdentPath == "" {
		return errors.New("invalid identities location for age")
	}
	return nil
}

func (c Config) SecretsType() string { return "age" }

func NewConfig(k *koanf.Koanf) (*Config, error) {
	k.All()
	var c Config
	c.IdentPath = k.String("idents")
	c.RepoPath = k.String("repo")
	return &c, nil
}

func (c *Config) Merge(other *Config) {
	if other.IdentPath != "" {
		c.IdentPath = other.IdentPath
	}
	if other.RepoPath != "" {
		c.RepoPath = other.RepoPath
	}
}
