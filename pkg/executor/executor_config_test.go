package executor

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/config"
)

func Test_NewExecutorConfig_TOML(t *testing.T) {
	clearMateriaEnv(t)
	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	assert.Nil(t, err)
	_, err = f.WriteString(`
[executor]
cleanup_components = true
materia_dir = "foo"
quadlet_dir = "bar"
scripts_dir = "/usr/bin"
service_dir = "/etc/systemd"
output_dir = ""
`)
	assert.Nil(t, err)
	err = f.Close()
	assert.Nil(t, err)

	k, err := config.Load(f.Name(), nil)
	assert.Nil(t, err)

	cfg, err := NewExecutorConfig(k)
	assert.Nil(t, err)

	assert.Equal(t, true, cfg.CleanupComponents)
	assert.Equal(t, "foo", cfg.MateriaDir)
	assert.Equal(t, "bar", cfg.QuadletDir)
	assert.Equal(t, "/usr/bin", cfg.ScriptsDir)
	assert.Equal(t, "", cfg.OutputDir)
	assert.Equal(t, "/etc/systemd", cfg.ServiceDir)
}

func Test_NewExecutorConfig_Env(t *testing.T) {
	clearMateriaEnv(t)
	t.Setenv("MATERIA_EXECUTOR__CLEANUP_COMPONENTS", "true")
	t.Setenv("MATERIA_EXECUTOR__MATERIA_DIR", "foo")
	t.Setenv("MATERIA_EXECUTOR__QUADLET_DIR", "bar")
	t.Setenv("MATERIA_EXECUTOR__SCRIPTS_DIR", "/usr/bin")
	t.Setenv("MATERIA_EXECUTOR__SERVICE_DIR", "/etc/systemd")
	t.Setenv("MATERIA_EXECUTOR__OUTPUT_DIR", "")

	k, err := config.Load("", nil)
	assert.Nil(t, err)

	cfg, err := NewExecutorConfig(k)
	assert.Nil(t, err)

	assert.Equal(t, true, cfg.CleanupComponents)
	assert.Equal(t, "foo", cfg.MateriaDir)
	assert.Equal(t, "bar", cfg.QuadletDir)
	assert.Equal(t, "/usr/bin", cfg.ScriptsDir)
	assert.Equal(t, "", cfg.OutputDir)
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

func Test_NewExecutorConfig_Delegate(t *testing.T) {
	clearMateriaEnv(t)
	k, err := config.Load("", nil)
	require.NoError(t, err)
	cfg, err := NewExecutorConfig(k)
	require.NoError(t, err)
	assert.Equal(t, "", cfg.DelegateDir, "unset means materia derives it from materia_dir")
	assert.Equal(t, 0, cfg.DelegateTimeout)
	assert.NoError(t, cfg.Validate())

	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	require.NoError(t, err)
	_, err = f.WriteString("[executor]\ndelegate_dir = \"/srv/delegate\"\ndelegate_timeout = 120\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	k, err = config.Load(f.Name(), nil)
	require.NoError(t, err)
	cfg, err = NewExecutorConfig(k)
	require.NoError(t, err)
	assert.Equal(t, "/srv/delegate", cfg.DelegateDir)
	assert.Equal(t, 120, cfg.DelegateTimeout)

	t.Setenv("MATERIA_EXECUTOR__DELEGATE_DIR", "/env/delegate")
	t.Setenv("MATERIA_EXECUTOR__DELEGATE_TIMEOUT", "-1")
	k, err = config.Load("", nil)
	require.NoError(t, err)
	cfg, err = NewExecutorConfig(k)
	require.NoError(t, err)
	assert.Equal(t, "/env/delegate", cfg.DelegateDir)
	assert.Error(t, cfg.Validate())
}
