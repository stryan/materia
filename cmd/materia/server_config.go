package main

import (
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/internal/materia"
)

type ServerConfig struct {
	PlanInterval   int    `koanf:"plan_interval" toml:"plan_interval"`
	UpdateInterval int    `koanf:"update_interval" toml:"update_interval"`
	Hostname       string `koanf:"hostname" toml:"hostname"`
	NotifyWebhook  string `koanf:"notify_webhook" toml:"notify_webhook"`
	UpdateWebhook  bool   `koanf:"update_webhook" toml:"update_webhook"`
	UpdateUrl      string `koanf:"update_url" toml:"update_url"`
	UpdateSecret   string `koanf:"update_secret" toml:"update_secret"`
	Socket         string `koanf:"socket" toml:"socket"`
}

type Server struct {
	syncSecret                   string
	Socket                       string
	UpdateInterval, PlanInterval int
	QuitOnError                  bool
	materia                      *materia.Materia
}

func (c ServerConfig) Validate() error {
	return nil
}

func NewConfig(k *koanf.Koanf) (*ServerConfig, error) {
	var c ServerConfig
	err := k.UnmarshalWithConf("server", &c, koanf.UnmarshalConf{})
	return &c, err
}
