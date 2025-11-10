package git

import (
	"fmt"

	"github.com/knadh/koanf/v2"
)

type Config struct {
	URL             string `toml:"URL" json:"URL" yaml:"URL"`
	Branch          string `toml:"branch" json:"branch" yaml:"branch"`
	PrivateKey      string `koanf:"private_key" toml:"private_key" json:"private_key" yaml:"private_key"`
	Username        string `toml:"username" json:"username" yaml:"username"`
	Password        string `toml:"password" json:"password" yaml:"password"`
	KnownHosts      string `toml:"known_hosts" json:"known_hosts" yaml:"known_hosts"`
	Insecure        bool   `koanf:"insecure" toml:"insecure" json:"insecure" yaml:"insecure"`
	LocalRepository string `toml:"local_repository" json:"local_repository" yaml:"local_repository"`
}

func NewConfig(k *koanf.Koanf, localDir, remoteURL string) (*Config, error) {
	var c Config

	c.Branch = k.String("git.branch")
	c.PrivateKey = k.String("git.private_key")
	c.Insecure = k.Bool("git.insecure")
	c.Username = k.String("git.username")
	c.Password = k.String("git.password")
	c.KnownHosts = k.String("git.knownhosts")
	c.LocalRepository = localDir
	c.URL = remoteURL
	return &c, nil
}

func (c *Config) String() string {
	var result string
	result += fmt.Sprintf("Branch: %v\n", c.Branch)
	result += fmt.Sprintf("KnownHosts: %v\n", c.KnownHosts)
	result += fmt.Sprintf("Allow Insecure: %v\n", c.Insecure)
	if c.PrivateKey != "" {
		result += fmt.Sprintf("PrivateKey file: %v\n", c.PrivateKey)
	}
	if c.Username != "" {
		result += fmt.Sprintf("Username: %v\n", c.Username)
	}
	return result
}
