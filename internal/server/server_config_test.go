package server

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/config"
)

var testConfig = &ServerConfig{
	PlanInterval:   60,
	UpdateInterval: 120,
	NotifyWebhook:  "webhook",
	UpdateWebhook:  true,
	FactsWebhook:   false,
	UpdateUrl:      "destination",
	Url:            "dest",
	UpdateSecret:   "secret",
	Token:          "token",
	Socket:         "/run/sock",
	QuitOnError:    false,
}

func Test_NewConfig_TOML(t *testing.T) {
	clearMateriaEnv(t)
	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	assert.Nil(t, err)
	_, err = f.WriteString(`
[server]
plan_interval = 60
update_interval = 120
hostname = "foo"
notify_webhook = "webhook"
update_webhook = true
facts_webhook = false
update_url = "destination"
url = "dest"
update_secret = "secret"
socket = "/run/sock"
token = "token"
`)
	assert.Nil(t, err)
	err = f.Close()
	assert.Nil(t, err)
	k, err := config.Load(f.Name(), nil)
	assert.Nil(t, err)

	cfg, err := NewConfig(k)
	assert.Nil(t, err)
	assert.Equal(t, testConfig, cfg)
}

func Test_NewConfig_Env(t *testing.T) {
	clearMateriaEnv(t)
	t.Setenv("MATERIA_SERVER__PLAN_INTERVAL", "60")
	t.Setenv("MATERIA_SERVER__UPDATE_INTERVAL", "120")
	t.Setenv("MATERIA_SERVER__HOSTNAME", "foo")
	t.Setenv("MATERIA_SERVER__NOTIFY_WEBHOOK", "webhook")
	t.Setenv("MATERIA_SERVER__URL", "dest")
	t.Setenv("MATERIA_SERVER__UPDATE_WEBHOOK", "true")
	t.Setenv("MATERIA_SERVER__FACTS_WEBHOOK", "false")
	t.Setenv("MATERIA_SERVER__UPDATE_URL", "destination")
	t.Setenv("MATERIA_SERVER__UPDATE_SECRET", "secret")
	t.Setenv("MATERIA_SERVER__TOKEN", "token")
	t.Setenv("MATERIA_SERVER__SOCKET", "/run/sock")

	k, err := config.Load("", nil)
	assert.Nil(t, err)

	cfg, err := NewConfig(k)
	assert.Nil(t, err)
	assert.Equal(t, testConfig, cfg, "Wanted %v got %v", testConfig, cfg)
}

func clearMateriaEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "MATERIA_") {
			t.Setenv(name, "")
			err := os.Unsetenv(name)
			require.Nil(t, err)
		}
	}
}
