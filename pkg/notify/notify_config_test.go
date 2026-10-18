package notify

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/config"
)

func Test_NewConfig_TOML(t *testing.T) {
	clearMateriaEnv(t)
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
	k, err := config.Load(f.Name(), nil)
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
	clearMateriaEnv(t)
	t.Setenv("MATERIA_NOTIFY__TRIGGERS__UPDATE", "https://example.com/webhook")
	t.Setenv("MATERIA_NOTIFY__TRIGGERS__ROLLBACK", "https://example.com/otherwebhook")

	k, err := config.Load("", nil)
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

	k, err := config.Load("", nil)
	assert.Nil(t, err)

	cfg, err := NewConfig(k)
	assert.Nil(t, err)
	assert.Error(t, cfg.Validate(), "expected invalid config")
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
