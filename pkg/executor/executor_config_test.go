package executor

import (
	"context"
	"os"
	"testing"

	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
	"primamateria.systems/materia/internal/config"
)

func Test_NewExecutorConfig_TOML(t *testing.T) {
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

	k, err := config.LoadConfigs(context.Background(), f.Name(), nil)
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
	t.Setenv("MATERIA_EXECUTOR__CLEANUP_COMPONENTS", "true")
	t.Setenv("MATERIA_EXECUTOR__MATERIA_DIR", "foo")
	t.Setenv("MATERIA_EXECUTOR__QUADLET_DIR", "bar")
	t.Setenv("MATERIA_EXECUTOR__SCRIPTS_DIR", "/usr/bin")
	t.Setenv("MATERIA_EXECUTOR__SERVICE_DIR", "/etc/systemd")
	t.Setenv("MATERIA_EXECUTOR__OUTPUT_DIR", "")

	k, err := config.LoadConfigs(context.Background(), "", nil)
	assert.Nil(t, err)

	cfg, err := NewExecutorConfig(k)
	assert.Nil(t, err)

	assert.Equal(t, true, cfg.CleanupComponents)
	assert.Equal(t, "foo", cfg.MateriaDir)
	assert.Equal(t, "bar", cfg.QuadletDir)
	assert.Equal(t, "/usr/bin", cfg.ScriptsDir)
	assert.Equal(t, "", cfg.OutputDir)
}

func TestNewExecutorConfigDefaultsInvalid(t *testing.T) {
	k := koanf.New(".")

	config, err := NewExecutorConfig(k)

	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.False(t, config.CleanupComponents)
	assert.Equal(t, "", config.MateriaDir)
	assert.Equal(t, "", config.QuadletDir)
	assert.Equal(t, "", config.ScriptsDir)
	assert.Equal(t, "", config.ServiceDir)
	assert.Equal(t, "", config.OutputDir)
	assert.Error(t, config.Validate(), "invalid config was valid")
}
