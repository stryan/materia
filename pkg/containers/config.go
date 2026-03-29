package containers

import (
	"fmt"
	"os"

	"github.com/knadh/koanf/v2"
)

type ContainersConfig struct {
	Remote        bool   `toml:"remote"`
	SecretsPrefix string `toml:"secrets_prefix"`
	Compression   string `toml:"compression"`
}

func NewContainersConfig(k *koanf.Koanf) (*ContainersConfig, error) {
	c := &ContainersConfig{}
	c.SecretsPrefix = k.String("containers.secrets_prefix")
	c.Compression = k.String("containers.compression")
	if k.Exists("containers.remote") {
		c.Remote = k.Bool("containers.remote")
	} else {
		c.Remote = (os.Getenv("container") == "podman")
	}
	if c.SecretsPrefix == "" {
		c.SecretsPrefix = "materia-"
	}
	if c.Compression != "" && (c.Compression != "zstd" && c.Compression != "gz" && c.Compression != "gzip") {
		return nil, fmt.Errorf("invalid compression type: %v", c.Compression)
	}
	return c, nil
}

func (c *ContainersConfig) String() string {
	return fmt.Sprintf("Remote: %v\n Secrets Prefix: %v\n Compression: %v\n", c.Remote, c.SecretsPrefix, c.Compression)
}
