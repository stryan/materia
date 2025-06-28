package git

import (
	"fmt"

	"github.com/knadh/koanf/v2"
)

type Config struct {
	Branch                            string
	PrivateKey                        string `koanf:"privatekey"`
	Username                          string
	Password                          string
	KnownHosts                        string
	Insecure                          bool `koanf:"insecure"`
	LocalRepository, RemoteRepository string
}

func NewConfig(k *koanf.Koanf, localDir, remote string) (*Config, error) {
	var c Config
	c.Branch = k.String("git.branch")
	c.PrivateKey = k.String("git.privatekey")
	c.Insecure = k.Bool("git.insecure")
	c.Username = k.String("git.username")
	c.Password = k.String("git.password")
	c.KnownHosts = k.String("git.knownhosts")
	c.LocalRepository = localDir
	c.RemoteRepository = remote
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
