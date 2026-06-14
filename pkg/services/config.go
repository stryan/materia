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
	c := &ServicesConfig{}
	c.Timeout = k.Int("services.timeout")
	c.DryrunQuadlets = k.Bool("dryrun_quadlets")
	if c.Timeout == 0 {
		c.Timeout = 90
	}
	if k.Exists("services.dbus_socket") {
		c.DbusSocket = k.String("services.dbus_socket")
	}
	return c, nil
}

func (c *ServicesConfig) String() string {
	return fmt.Sprintf("Default Timeout: %v\nDry Run Quadlets: %v", c.Timeout, c.DryrunQuadlets)
}
