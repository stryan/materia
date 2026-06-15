package notify

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"primamateria.systems/materia/internal/config"
)

func Test_NewConfig_TOML(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	assert.Nil(t, err)
	_, err = f.WriteString(`
[notify.triggers]
update = "https://example.com/webhook"
rollback = "https://example.com/otherwebhook"
`)
	assert.Nil(t, err)
	err = f.Close()
	assert.Nil(t, err)
	k, err := config.LoadConfigs(context.Background(), f.Name(), nil)
	assert.Nil(t, err)

	cfg, err := NewConfig(k)
	assert.Nil(t, err)

	expected := map[string]string{
		"update":   "https://example.com/webhook",
		"rollback": "https://example.com/otherwebhook",
	}

	for key, want := range expected {
		got := cfg.Triggers[key]
		assert.Equal(t, want, got)
	}
}

func Test_NewConfig_Env(t *testing.T) {
	t.Setenv("MATERIA_NOTIFY__TRIGGERS__UPDATE", "https://example.com/webhook")
	t.Setenv("MATERIA_NOTIFY__TRIGGERS__ROLLBACK", "https://example.com/otherwebhook")

	k, err := config.LoadConfigs(context.Background(), "", nil)
	assert.Nil(t, err)

	cfg, err := NewConfig(k)
	assert.Nil(t, err)

	expected := map[string]string{
		"update":   "https://example.com/webhook",
		"rollback": "https://example.com/otherwebhook",
	}
	for key, want := range expected {
		got := cfg.Triggers[key]
		assert.Equal(t, want, got)
	}
}

func Test_InvalidConfig(t *testing.T) {
	t.Setenv("MATERIA_NOTIFY__TRIGGERS__UPDATE", "https://example.com/webhook")
	t.Setenv("MATERIA_NOTIFY__TRIGGERS__FORFUN", "https://example.com/otherwebhook")

	k, err := config.LoadConfigs(context.Background(), "", nil)
	assert.Nil(t, err)

	cfg, err := NewConfig(k)
	assert.Nil(t, err)
	assert.Error(t, cfg.Validate(), "expected invalid config")
}
