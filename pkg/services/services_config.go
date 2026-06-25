package services

import (
	"fmt"

	"github.com/knadh/koanf/v2"
)

type ServicesConfig struct {
	Timeout        int    `toml:"timeout" koanf:"timeout"`
	DryrunQuadlets bool   `toml:"dryrun_quadlets" koanf:"dryrun_quadlets"`
	DbusSocket     string `toml:"dbus_socket" koanf:"dbus_socket"`
}

func NewServicesConfig(k *koanf.Koanf) (*ServicesConfig, error) {
	c := DefaultServicesConfig()
	err := k.UnmarshalWithConf("services", c, koanf.UnmarshalConf{})
	if err != nil {
		return nil, fmt.Errorf("unable to create services config: %w", err)
	}
	return c, nil
}

func (c *ServicesConfig) String() string {
	return fmt.Sprintf("Default Timeout: %v\nDry Run Quadlets: %v", c.Timeout, c.DryrunQuadlets)
}

func DefaultServicesConfig() *ServicesConfig {
	return &ServicesConfig{
		Timeout:        90,
		DryrunQuadlets: false,
		DbusSocket:     "",
	}
}
