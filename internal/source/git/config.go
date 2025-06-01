package git

import (
	"fmt"

	"github.com/knadh/koanf/v2"
)

type Config struct {
	Branch     string
	PrivateKey string `koanf:"privatekey"`
	Username   string
	Password   string
	KnownHosts string
	Insecure   bool `koanf:"insecure"`
}

func NewConfig(k *koanf.Koanf) (*Config, error) {
	var c Config
	c.Branch = k.String("branch")
	c.PrivateKey = k.String("privatekey")
	c.Insecure = k.Bool("insecure")
	c.Username = k.String("username")
	c.Password = k.String("password")
	c.KnownHosts = k.String("knownhosts")
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
