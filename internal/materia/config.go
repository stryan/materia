package materia

import (
	"errors"
	"strings"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	GitRepo  string
	Debug    bool
	Hostname string
	Timeout  int
}

func NewConfig() (*Config, error) {
	k := koanf.New(".")
	err := k.Load(env.Provider("MATERIA", ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, "MATERIA")), "_", ".", -1)
	}), nil)
	if err != nil {
		return nil, err
	}
	var c Config
	c.GitRepo = k.String(".gitrepo")
	c.Debug = k.Bool(".debug")
	c.Hostname = k.String(".hostname")
	c.Timeout = k.Int(".timeout")

	return &c, nil
}

func (c *Config) Validate() error {
	if c.GitRepo == "" {
		return errors.New("need git repo location")
	}
	return nil
}
