package planner

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/config"
)

func Test_NewPlannerConfig_TOML(t *testing.T) {
	clearMateriaEnv(t)
	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	assert.Nil(t, err)
	_, err = f.WriteString(`
[planner]
only_resources = true
cleanup_quadlets = true
cleanup_volumes = true
backup_volumes = false
migrate_volumes = true
`)
	assert.Nil(t, err)
	err = f.Close()
	assert.Nil(t, err)

	k, err := config.Load(f.Name(), nil)
	assert.Nil(t, err)

	cfg, err := NewPlannerConfig(k)
	assert.Nil(t, err)

	assert.Equal(t, true, cfg.OnlyResources)
	assert.Equal(t, true, cfg.CleanupQuadlets)
	assert.Equal(t, true, cfg.CleanupVolumes)
	assert.Equal(t, false, cfg.BackupVolumes)
	assert.Equal(t, true, cfg.MigrateVolumes)
}

func Test_NewPlannerConfig_Env(t *testing.T) {
	clearMateriaEnv(t)
	t.Setenv("MATERIA_PLANNER__ONLY_RESOURCES", "true")
	t.Setenv("MATERIA_PLANNER__CLEANUP_QUADLETS", "true")
	t.Setenv("MATERIA_PLANNER__CLEANUP_VOLUMES", "true")
	t.Setenv("MATERIA_PLANNER__BACKUP_VOLUMES", "false")
	t.Setenv("MATERIA_PLANNER__MIGRATE_VOLUMES", "true")

	k, err := config.Load("", nil)
	assert.Nil(t, err)

	cfg, err := NewPlannerConfig(k)
	assert.Nil(t, err)

	assert.Equal(t, true, cfg.OnlyResources)
	assert.Equal(t, true, cfg.CleanupQuadlets)
	assert.Equal(t, true, cfg.CleanupVolumes)
	assert.Equal(t, false, cfg.BackupVolumes)
	assert.Equal(t, true, cfg.MigrateVolumes)
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
