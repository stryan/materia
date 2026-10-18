package server

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/log/v2"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/internal/rpc"
)

type ServerConfig struct {
	PlanInterval   int    `koanf:"plan_interval" toml:"plan_interval"`
	UpdateInterval int    `koanf:"update_interval" toml:"update_interval"`
	NotifyWebhook  string `koanf:"notify_webhook" toml:"notify_webhook"`
	UpdateWebhook  bool   `koanf:"update_webhook" toml:"update_webhook"`
	FactsWebhook   bool   `koanf:"facts_webhook" toml:"facts_webhook"`
	Health         bool   `koanf:"health" toml:"health"`
	UpdateUrl      string `koanf:"update_url" toml:"update_url"`
	Url            string `koanf:"url" toml:"url"`
	UpdateSecret   string `koanf:"update_secret" toml:"update_secret"`
	Token          string `koanf:"token" toml:"token"`
	Socket         string `koanf:"socket" toml:"socket"`
	QuitOnError    bool   `koanf:"quit_on_error" toml:"quit_on_error"`
}

func (c ServerConfig) Validate() error {
	errs := []error{}
	if c.Socket != "" && !strings.HasPrefix(c.Socket, "unix:") && !strings.HasPrefix(c.Socket, "tcp:") {
		errs = append(errs, fmt.Errorf("invalid socket type for %v; should be unix or tcp", c.Socket))
	}
	updateHookProtected := (c.UpdateSecret != "") || (c.Token != "")
	factsHookProtected := (c.Token != "")
	isLocalHost := strings.HasPrefix(c.Url, "localhost")

	if c.FactsWebhook && !factsHookProtected && !isLocalHost {
		errs = append(errs, errors.New("facts webhook enabled with no security and not on localhost"))
	}
	if c.UpdateWebhook && !updateHookProtected && !isLocalHost {
		errs = append(errs, errors.New("update webhook enabled with no security and not on localhost"))
	}
	if c.UpdateSecret != "" {
		log.Warn("DEPRECATED: server.update_secret should be replaced with server.token")
	}
	if c.UpdateUrl != "" {
		log.Warn("DEPRECATED: server.update_url should be replaced with server.url")
	}
	return errors.Join(errs...)
}

func NewConfig(k *koanf.Koanf) (*ServerConfig, error) {
	c, err := DefaultServicesConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to generate default server config: %w", err)
	}
	err = k.UnmarshalWithConf("server", c, koanf.UnmarshalConf{})
	if c.UpdateUrl != "" && c.Url == "" {
		c.Url = c.UpdateUrl
	}
	if c.FactsWebhook || c.UpdateWebhook {
		c.Health = true
	}
	return c, err
}

func (c ServerConfig) HasWebhook() bool {
	return c.FactsWebhook || c.Health || c.UpdateWebhook
}

func DefaultServicesConfig() (*ServerConfig, error) {
	socket, err := rpc.SocketPath()
	if err != nil {
		return nil, err
	}
	return &ServerConfig{
		Socket: socket,
	}, nil
}
