package executor

import (
	"testing"

	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
)

func TestNewExecutorConfig(t *testing.T) {
	k := koanf.New(".")

	assert.NoError(t, k.Set("executor.cleanup_components", true))
	assert.NoError(t, k.Set("executor.materia_dir", "/test/materia"))
	assert.NoError(t, k.Set("executor.quadlet_dir", "/test/quadlet"))
	assert.NoError(t, k.Set("executor.scripts_dir", "/test/scripts"))
	assert.NoError(t, k.Set("executor.service_dir", "/test/service"))
	assert.NoError(t, k.Set("executor.output_dir", "/test/output"))

	config, err := NewExecutorConfig(k)

	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.True(t, config.CleanupComponents)
	assert.Equal(t, "/test/materia", config.MateriaDir)
	assert.Equal(t, "/test/quadlet", config.QuadletDir)
	assert.Equal(t, "/test/scripts", config.ScriptsDir)
	assert.Equal(t, "/test/service", config.ServiceDir)
	assert.Equal(t, "/test/output", config.OutputDir)
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
