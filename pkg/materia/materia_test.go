package materia

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/attributes/mem"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/executor"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/mocks"
)

func TestNew(t *testing.T) {
	hm := mocks.NewMockHostManager(t)
	sm := mocks.NewMockSourceManager(t)
	sm.EXPECT().LoadManifest(manifests.MateriaManifestFile).Return(&manifests.MateriaManifest{}, nil)
	engine := mem.NewMemoryEngine()
	tmpdir := t.TempDir()
	m, err := New(context.Background(), &MateriaConfig{
		QuadletDir: filepath.Join(tmpdir, "/tmp/materia/quadlets"),
		MateriaDir: filepath.Join(tmpdir, "/tmp/materia"),
		ServiceDir: filepath.Join(tmpdir, "/tmp/services"),
		ScriptsDir: filepath.Join(tmpdir, "/usr/local/bin"),
		SourceDir:  filepath.Join(tmpdir, "/materia/source"),
		OutputDir:  filepath.Join(tmpdir, "/materia/output"),
		RemoteDir:  filepath.Join(tmpdir, "/materia/source/remote"),
	}, hm, sm, engine)
	assert.NoError(t, err)
	assert.NotNil(t, m)
}

func TestNew_PathOverrides(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)
	data := filepath.Join(t.TempDir(), "data")
	const (
		quadlets = "/local/quadlets"
		scripts  = "/local/scripts"
		services = "/local/services"
	)

	tests := []struct {
		name     string
		rootless bool
		mounts   map[string]containers.ContainerMount
		override executor.ExecutorConfig
		want     executor.ExecutorConfig
	}{
		{
			name: "hardcoded dirs",
			want: executor.ExecutorConfig{MateriaDir: data, QuadletDir: quadlets, ScriptsDir: scripts, ServiceDir: services, OutputDir: data + "/output"},
		},
		{
			name:     "mounts from container",
			rootless: true,
			mounts:   map[string]containers.ContainerMount{data: {Source: "/host/data"}, quadlets: {Source: "/host/quadlets"}},
			want:     executor.ExecutorConfig{MateriaDir: "/host/data", QuadletDir: "/host/quadlets", ScriptsDir: scripts, ServiceDir: services, OutputDir: "/host/data/output"},
		},
		{
			name:     "user override",
			rootless: true,
			mounts:   map[string]containers.ContainerMount{data: {Source: "/host/data"}},
			override: executor.ExecutorConfig{MateriaDir: "/override/data"},
			want:     executor.ExecutorConfig{MateriaDir: "/override/data", QuadletDir: quadlets, ScriptsDir: scripts, ServiceDir: services, OutputDir: "/override/data/output"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hm := mocks.NewMockHostManager(t)
			if tc.rootless {
				hm.EXPECT().ListContainers(mock.Anything, mock.Anything).
					Return([]*containers.Container{{Hostname: hostname, BindMounts: tc.mounts}}, nil)
			}
			sm := mocks.NewMockSourceManager(t)
			sm.EXPECT().LoadManifest(manifests.MateriaManifestFile).Return(&manifests.MateriaManifest{}, nil)

			m, err := New(context.Background(), &MateriaConfig{
				MateriaDir: data, QuadletDir: quadlets, ScriptsDir: scripts, ServiceDir: services,
				OutputDir: data + "/output", SourceDir: data + "/source", RemoteDir: data + "/remote",
				Rootless: tc.rootless, ExecutorConfig: tc.override,
			}, hm, sm, mem.NewMemoryEngine())
			require.NoError(t, err)

			assert.Equal(t, tc.want, m.Executor.ExecutorConfig)
		})
	}
}
