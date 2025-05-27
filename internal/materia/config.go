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
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	SourceURL     string
	Debug         bool
	UseStdout     bool
	Diffs         bool
	Cleanup       bool
	Hostname      string
	Roles         []string
	Timeout       int
	MateriaDir    string
	QuadletDir    string
	ServiceDir    string
	ScriptDir     string
	SourceDir     string
	OutputDir     string
	OnlyResources bool
	Quiet         bool
	GitConfig     *git.Config
	AgeConfig     *age.Config
	User          *user.User
}

func NewConfig(configFile string) (*Config, error) {
	k := koanf.New(".")
	err := k.Load(env.Provider("MATERIA", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(
			strings.TrimPrefix(s, "MATERIA_")), "_", ".")
	}), nil)
	if err != nil {
		return nil, fmt.Errorf("error loading config from env: %w", err)
	}
	if configFile != "" {
		err = k.Load(file.Provider(configFile), toml.Parser())
		if err != nil {
			return nil, fmt.Errorf("error loading config file: %w", err)
		}
	}
	var c Config
	c.SourceURL = k.String("sourceurl")
	c.SourceDir = k.String("source")
	c.Debug = k.Bool("debug")
	c.Cleanup = k.Bool("cleanup")
	c.Hostname = k.String("hostname")
	c.Timeout = k.Int("timeout")
	c.Roles = k.Strings("roles")
	c.Diffs = k.Bool("diffs")
	c.UseStdout = k.Bool("stdout")
	c.MateriaDir = k.String("prefix")
	c.QuadletDir = k.String("destination")
	c.ServiceDir = k.String("services")
	c.ScriptDir = k.String("scripts")
	c.OutputDir = k.String("output")
	if k.Exists("git") {
		c.GitConfig, err = git.NewConfig(k.Cut("git"))
		if err != nil {
			return nil, err
		}
	}
	if k.Exists("age") {
		c.AgeConfig, err = age.NewConfig(k.Cut("age"))
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
	dataPath := "/var/lib"
	quadletPath := "/etc/containers/systemd/"
	// TODO once we can determine whether /var and /root are on the same filesystem switch this to a /var/lib/materia path and systemctl-link them in
	// otherwise, defer to usual /etc location to work out of the box with MicroOS
	servicePath := "/etc/systemd/system/"
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
			dataPath = fmt.Sprintf("%v/.local/share", home)
			servicePath = fmt.Sprintf("%v/.local/share/systemd/user", home)
		} else {
			dataPath = datadir
			servicePath = fmt.Sprintf("%v/systemd/user", datadir)
		}
	}
	if c.MateriaDir == "" {
		c.MateriaDir = dataPath
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
		c.SourceDir = filepath.Join(dataPath, "materia", "source")
	}
	if c.OutputDir == "" {
		c.OutputDir = filepath.Join(dataPath, "materia", "output")
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

func (c *Config) String() string {
	var result string
	result += fmt.Sprintf("Source URL: %v\n", c.SourceURL)
	result += fmt.Sprintf("Debug mode: %v\n", c.Debug)
	result += fmt.Sprintf("STDOUT: %v\n", c.UseStdout)
	result += fmt.Sprintf("Show Diffs: %v\n", c.Diffs)
	result += fmt.Sprintf("Cleanup: %v\n", c.Cleanup)
	result += fmt.Sprintf("Hostname: %v\n", c.Hostname)
	result += fmt.Sprintf("Configured Roles: %v\n", c.Roles)
	result += fmt.Sprintf("Service Timeout: %v\n", c.Timeout)
	result += fmt.Sprintf("Materia Root: %v\n", c.MateriaDir)
	result += fmt.Sprintf("Quadlet Dir: %v\n", c.QuadletDir)
	result += fmt.Sprintf("Scripts Dir: %v\n", c.ScriptDir)
	result += fmt.Sprintf("Source cache dir: %v\n", c.SourceDir)
	result += fmt.Sprintf("User: %v\n", c.User.Username)
	if c.GitConfig != nil {
		result += "Using git\n"
		result += c.GitConfig.String()
	}
	if c.AgeConfig != nil {
		result += "Secrets Engine: age\n"
		result += c.AgeConfig.String()
	}
	return result
}
