package materia

import (
	"errors"
	"os/user"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	"git.saintnet.tech/stryan/materia/internal/source/git"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	SourceURL   string
	Debug       bool
	UseStdout   bool
	Diffs       bool
	Hostname    string
	Roles       []string
	Timeout     int
	Prefix      string
	Destination string
	Services    string
	GitConfig   *git.Config
	AgeConfig   *age.Config
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
	k.All()
	var c Config
	c.SourceURL = k.String(".sourceurl")
	c.Debug = k.Bool(".debug")
	c.Hostname = k.String(".hostname")
	c.Timeout = k.Int(".timeout")
	c.Prefix = k.String(".prefix")
	c.Roles = k.Strings(".roles")
	c.Diffs = k.Bool(".diffs")
	c.UseStdout = k.Bool(".stdout")
	c.Destination = k.String(".destination")
	c.Services = k.String(".services")
	if k.Exists(".git") {
		c.GitConfig, err = git.NewConfig(k.Cut(".git"))
		if err != nil {
			return nil, err
		}
	}
	if k.Exists(".age") {
		c.AgeConfig, err = age.NewConfig(k.Cut(".age"))
		if err != nil {
			return nil, err
		}
	}
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
