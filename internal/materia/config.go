package materia

import (
	"errors"
	"strings"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	SourceURL   string
	Debug       bool
	Hostname    string
	Timeout     int
	Prefix      string
	Destination string
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
	c.SourceURL = k.String(".sourceurl")
	c.Debug = k.Bool(".debug")
	c.Hostname = k.String(".hostname")
	c.Timeout = k.Int(".timeout")
	c.Prefix = k.String(".prefix")
	c.Destination = k.String(".destination")

	return &c, nil
}

func (c *Config) Validate() error {
	if c.SourceURL == "" {
		return errors.New("need source location")
	}
	return nil
}
