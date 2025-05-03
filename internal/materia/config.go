package materia

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	"git.saintnet.tech/stryan/materia/internal/source/git"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	SourceURL  string
	Debug      bool
	UseStdout  bool
	Diffs      bool
	Cleanup    bool
	Hostname   string
	Roles      []string
	Timeout    int
	MateriaDir string
	QuadletDir string
	ServiceDir string
	ScriptDir  string
	SourceDir  string
	GitConfig  *git.Config
	AgeConfig  *age.Config
	User       *user.User
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
	c.SourceDir = k.String(".source")
	c.Debug = k.Bool(".debug")
	c.Cleanup = k.Bool(".cleanup")
	c.Hostname = k.String(".hostname")
	c.Timeout = k.Int(".timeout")
	c.Roles = k.Strings(".roles")
	c.Diffs = k.Bool(".diffs")
	c.UseStdout = k.Bool(".stdout")
	c.MateriaDir = k.String(".prefix")
	c.QuadletDir = k.String(".destination")
	c.ServiceDir = k.String(".services")
	c.ScriptDir = k.String(".scripts")
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

	// calculate defaults
	if c.MateriaDir == "" {
		c.MateriaDir = "/var/lib"
	}
	quadletPath := "/etc/containers/systemd/"
	servicePath := "/usr/local/lib/systemd/system/"
	scriptsPath := "/usr/local/bin"

	if c.User.Username != "root" {
		home := c.User.HomeDir
		var found bool
		conf, found := os.LookupEnv("XDG_CONFIG_HOME")
		if !found {
			quadletPath = fmt.Sprintf("%v/.config/containers/systemd/", home)
		} else {
			quadletPath = fmt.Sprintf("%v/containers/systemd/", conf)
		}
		datadir, found := os.LookupEnv("XDG_DATA_HOME")
		if !found {
			servicePath = fmt.Sprintf("%v/.local/share/systemd/user", home)
		} else {
			servicePath = fmt.Sprintf("%v/systemd/user", datadir)
		}
	}

	if c.QuadletDir == "" {
		c.QuadletDir = quadletPath
	}
	if c.ServiceDir == "" {
		c.ServiceDir = servicePath
	}
	if c.ScriptDir == "" {
		c.ScriptDir = scriptsPath
	}
	if c.SourceDir == "" {
		c.SourceDir = filepath.Join(c.MateriaDir, "materia", "source")
	}

	return &c, nil
}

func (c *Config) Validate() error {
	if c.SourceURL == "" {
		return errors.New("need source location")
	}
	if c.QuadletDir == "" {
		return errors.New("need quadlet directory")
	}
	if c.ServiceDir == "" {
		return errors.New("need services directory")
	}
	if c.ScriptDir == "" {
		return errors.New("need scripts directory")
	}
	if c.SourceDir == "" {
		return errors.New("need source directory")
	}
	return nil
}
