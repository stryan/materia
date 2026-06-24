package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"primamateria.systems/materia/internal/config"
)

var testConfig = &ServerConfig{
	60,
	120,
	"foo",
	"webhook",
	true,
	"destination",
	"secret",
	"/run/sock",
}

func Test_NewConfig_TOML(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	assert.Nil(t, err)
	_, err = f.WriteString(`
[server]
plan_interval = 60
update_interval = 120
hostname = "foo"
notify_webhook = "webhook"
update_webhook = true
update_url = "destination"
update_secret = "secret"
socket = "/run/sock"
`)
	assert.Nil(t, err)
	err = f.Close()
	assert.Nil(t, err)
	k, err := config.LoadConfigs(context.Background(), f.Name(), nil)
	assert.Nil(t, err)

	cfg, err := NewConfig(k)
	assert.Nil(t, err)
	assert.Equal(t, cfg, testConfig)
}

func Test_NewConfig_Env(t *testing.T) {
	t.Setenv("MATERIA_SERVER__PLAN_INTERVAL", "60")
	t.Setenv("MATERIA_SERVER__UPDATE_INTERVAL", "120")
	t.Setenv("MATERIA_SERVER__HOSTNAME", "foo")
	t.Setenv("MATERIA_SERVER__NOTIFY_WEBHOOK", "webhook")
	t.Setenv("MATERIA_SERVER__UPDATE_WEBHOOK", "true")
	t.Setenv("MATERIA_SERVER__UPDATE_URL", "destination")
	t.Setenv("MATERIA_SERVER__UPDATE_SECRET", "secret")
	t.Setenv("MATERIA_SERVER__SOCKET", "/run/sock")

	k, err := config.LoadConfigs(context.Background(), "", nil)
	assert.Nil(t, err)

	cfg, err := NewConfig(k)
	assert.Nil(t, err)
	assert.Equal(t, cfg, testConfig)
}
