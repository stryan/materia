package containers

import (
	"fmt"
	"os"

	"github.com/knadh/koanf/v2"
)

type ContainersConfig struct {
	Remote        bool   `koanf:"remote"`
	SecretsPrefix string `koanf:"secrets_prefix"`
	Compression   string `koanf:"compression"`
}

func NewContainersConfig(k *koanf.Koanf) (*ContainersConfig, error) {
	c := DefaultContainersConfig()
	err := k.UnmarshalWithConf("containers", c, koanf.UnmarshalConf{})
	if err != nil {
		return nil, fmt.Errorf("unable to create containers config: %w", err)
	}
	return c, nil
}

func (c *ContainersConfig) String() string {
	return fmt.Sprintf("Remote: %v\n Secrets Prefix: %v\n Compression: %v\n", c.Remote, c.SecretsPrefix, c.Compression)
}

func DefaultContainersConfig() *ContainersConfig {
	return &ContainersConfig{
		Remote:        (os.Getenv("container") == "podman"),
		SecretsPrefix: "materia-",
		Compression:   "",
	}
}

func (c *ContainersConfig) Validate() error {
	if c.Compression != "" && (c.Compression != "zstd" && c.Compression != "gz" && c.Compression != "gzip") {
		return fmt.Errorf("invalid compression type: %v", c.Compression)
	}
	return nil
}
