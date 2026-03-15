package containers

import (
	"fmt"
	"os"

	"github.com/knadh/koanf/v2"
)

type ContainersConfig struct {
	Remote             bool   `toml:"remote"`
	SecretsPrefix      string `toml:"secrets_prefix"`
	CompressionCommand string `toml:"compression_command"`
	CompressionSuffix  string `toml:"compression_suffix"`
}

func NewContainersConfig(k *koanf.Koanf) (*ContainersConfig, error) {
	c := &ContainersConfig{}
	c.SecretsPrefix = k.String("containers.secrets_prefix")
	c.CompressionCommand = k.String("containers.compression_command")
	c.CompressionSuffix = k.String("containers.compression_suffix")
	if k.Exists("containers.remote") {
		c.Remote = k.Bool("containers.remote")
	} else {
		c.Remote = (os.Getenv("container") == "podman")
	}
	if c.SecretsPrefix == "" {
		c.SecretsPrefix = "materia-"
	}
	if c.CompressionSuffix == "" {
		switch c.CompressionCommand {
		case "zstd":
			c.CompressionSuffix = "zstd"
		case "gzip":
			c.CompressionSuffix = "gz"
		case "zip":
			c.CompressionSuffix = "zip"
		default:
			c.CompressionSuffix = "compressed"
		}
	}
	return c, nil
}

func (c *ContainersConfig) String() string {
	return fmt.Sprintf("Remote: %v\n Secrets Prefix: %v\n CompressionCommand: %v\n CompressionSuffix: %v\n", c.Remote, c.SecretsPrefix, c.CompressionCommand, c.CompressionSuffix)
}
