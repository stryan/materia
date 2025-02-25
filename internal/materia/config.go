package materia

import (
	"errors"
	"os/user"
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
	Services    string
	PrivateKey  string
	User        *user.User
}

func NewConfig() (*Config, error) {
	k := koanf.New(".")
	err := k.Load(env.Provider("MATERIA", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(
			strings.TrimPrefix(s, "MATERIA")), "_", ".")
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
	c.Services = k.String(".services")
	c.PrivateKey = k.String(".privatekey")
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	c.User = currentUser

	return &c, nil
}

func (c *Config) Validate() error {
	if c.SourceURL == "" {
		return errors.New("need source location")
	}
	return nil
}
