package containers

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"primamateria.systems/materia/internal/config"
)

func Test_NewContainersConfig_TOML(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "*.toml")
	assert.Nil(t, err)
	_, err = f.WriteString(`
[containers]
remote = true
secrets_prefix = "custom-"
compression = "zstd"
`)
	assert.Nil(t, err)
	err = f.Close()
	assert.Nil(t, err)

	k, err := config.LoadConfigs(context.Background(), f.Name(), nil)
	assert.Nil(t, err)

	cfg, err := NewContainersConfig(k)
	assert.Nil(t, err)

	assert.Equal(t, true, cfg.Remote)
	assert.Equal(t, "custom-", cfg.SecretsPrefix)
	assert.Equal(t, "zstd", cfg.Compression)
}

func Test_NewContainersConfig_Env(t *testing.T) {
	t.Setenv("MATERIA_CONTAINERS__REMOTE", "true")
	t.Setenv("MATERIA_CONTAINERS__SECRETS_PREFIX", "custom-")
	t.Setenv("MATERIA_CONTAINERS__COMPRESSION", "zstd")

	k, err := config.LoadConfigs(context.Background(), "", nil)
	assert.Nil(t, err)

	cfg, err := NewContainersConfig(k)
	assert.Nil(t, err)

	assert.Equal(t, true, cfg.Remote)
	assert.Equal(t, "custom-", cfg.SecretsPrefix)
	assert.Equal(t, "zstd", cfg.Compression)
}

func Test_NewContainersConfig_Defaults(t *testing.T) {
	t.Setenv("container", "")

	k, err := config.LoadConfigs(context.Background(), "", nil)
	assert.Nil(t, err)

	cfg, err := NewContainersConfig(k)
	assert.Nil(t, err)

	assert.Equal(t, false, cfg.Remote)
	assert.Equal(t, "materia-", cfg.SecretsPrefix)
	assert.Equal(t, "", cfg.Compression)
}

func Test_NewContainersConfig_RemoteDefaultFromEnv(t *testing.T) {
	t.Setenv("container", "podman")

	k, err := config.LoadConfigs(context.Background(), "", nil)
	assert.Nil(t, err)

	cfg, err := NewContainersConfig(k)
	assert.Nil(t, err)

	assert.Equal(t, true, cfg.Remote)
}

func Test_ContainersConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ContainersConfig
		wantErr bool
	}{
		{name: "happy-path-zstd", cfg: ContainersConfig{Compression: "zstd"}, wantErr: false},
		{name: "happy-path-gz", cfg: ContainersConfig{Compression: "gz"}, wantErr: false},
		{name: "happy-path-gzip", cfg: ContainersConfig{Compression: "gzip"}, wantErr: false},
		{name: "happy-path-empty", cfg: ContainersConfig{Compression: ""}, wantErr: false},
		{name: "sad-path-zip", cfg: ContainersConfig{Compression: "zip"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				assert.Error(t, tt.cfg.Validate())
			} else {
				assert.NoError(t, tt.cfg.Validate())
			}
		})
	}
}
