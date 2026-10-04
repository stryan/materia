package sourceman

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/mocks"
	"primamateria.systems/materia/pkg/repository"
	"primamateria.systems/materia/pkg/source"
	"primamateria.systems/materia/pkg/source/local"
	"primamateria.systems/materia/pkg/source/oci"
)

func mockMaker(t *testing.T, ms source.Source) func(_ manifests.RemoteComponentConfig, _ string) (source.Source, error) {
	t.Helper()
	return func(_ manifests.RemoteComponentConfig, _ string) (source.Source, error) {
		return ms, nil
	}
}

func Test_Sourceman_LoadRemotes_NoRemotes(t *testing.T) {
	base := t.TempDir()
	dir, remote := filepath.Join(base, "source"), filepath.Join(base, "remote")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "components"), 0o755))
	srcman, err := NewSourceManager(&SourceManConfig{dir, remote})
	require.NoError(t, err)
	ms := mocks.NewMockSource(t)
	srcman.maker = mockMaker(t, ms)
	man := manifests.MateriaManifest{}
	var buf bytes.Buffer

	require.NoError(t, toml.NewEncoder(&buf).Encode(man))

	require.NoError(t, os.WriteFile(filepath.Join(dir, manifests.MateriaManifestFile), buf.Bytes(), 0o644))
	require.NoError(t, srcman.LoadRemotes(context.Background()))
	require.Equal(t, 0, len(srcman.sources))
}

func Test_Sourceman_LoadRemotes_EmptyRemoteDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "source")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "components"), 0o755))
	srcman, err := NewSourceManager(&SourceManConfig{dir, ""})
	require.NoError(t, err)
	ms := mocks.NewMockSource(t)
	srcman.maker = mockMaker(t, ms)
	man := manifests.MateriaManifest{
		Remotes: map[string]manifests.RemoteComponentConfig{
			"test": {},
		},
	}
	var buf bytes.Buffer

	require.NoError(t, toml.NewEncoder(&buf).Encode(man))

	require.NoError(t, os.WriteFile(filepath.Join(dir, manifests.MateriaManifestFile), buf.Bytes(), 0o644))
	require.ErrorIs(t, srcman.LoadRemotes(context.Background()), repository.ErrNoRemoteDir)
}

func Test_Sourceman_LoadRemotes_OneEach(t *testing.T) {
	base := t.TempDir()
	dir, remote := filepath.Join(base, "source"), filepath.Join(base, "remote")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "components"), 0o755))
	srcman, err := NewSourceManager(&SourceManConfig{dir, remote})
	require.NoError(t, err)
	psource := mocks.NewMockSource(t)
	require.NoError(t, srcman.AddSource(psource, &source.SyncOpts{}, &source.SyncReport{}, true))
	ms := mocks.NewMockSource(t)

	require.NoError(t, os.MkdirAll(filepath.Join(remote, "components", "test"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(remote, "components", "test", manifests.MateriaManifestFile), []byte(""), 0o644))
	ms.EXPECT().Sync(mock.Anything, source.SyncOpts{}).Twice().Return(&source.SyncReport{}, nil)

	srcman.maker = mockMaker(t, ms)
	man := manifests.MateriaManifest{
		Remotes: map[string]manifests.RemoteComponentConfig{
			"test": {
				FileSource: &local.Config{},
				Subpath:    "",
				Revision:   "",
			},
		},
	}
	var buf bytes.Buffer

	require.NoError(t, toml.NewEncoder(&buf).Encode(man))
	require.NoError(t, os.WriteFile(filepath.Join(dir, manifests.MateriaManifestFile), buf.Bytes(), 0o644))

	require.NoError(t, srcman.LoadRemotes(context.Background()))
	require.Equal(t, 2, len(srcman.sources))
	require.NoError(t, srcman.LoadRemotes(context.Background()))
	require.Equal(t, 2, len(srcman.sources))

	buf.Reset()
	man = manifests.MateriaManifest{
		Remotes: map[string]manifests.RemoteComponentConfig{},
	}

	require.NoError(t, toml.NewEncoder(&buf).Encode(man))

	require.NoError(t, os.WriteFile(filepath.Join(dir, manifests.MateriaManifestFile), buf.Bytes(), 0o644))

	require.NoError(t, srcman.LoadRemotes(context.Background()))
	require.Equal(t, 1, len(srcman.sources))
}

// TestSyncRemotesOCIConstruction verifies the TOML → koanf → NewOCISource
// chain that SyncRemotes uses. LoadMateriaManifest must deserialize [Remotes]
// into non-nil source configs (koanf struct tags), and NewOCISource must
// parse the URL into Registry/Repository (ParseURL fix).
func TestSyncRemotesOCIConstruction(t *testing.T) {
	sourceDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(sourceDir, manifests.MateriaManifestFile),
		[]byte(`
[Remotes.caddy.oci]
url = "oci://git.example.com/user/materia-caddy"
tag = "2026-03-06"
username = "user"
password = "token"
`), 0o644,
	))

	// Step 1: same as SyncRemotes line 70
	man, err := manifests.LoadMateriaManifest(
		filepath.Join(sourceDir, manifests.MateriaManifestFile),
	)
	require.NoError(t, err)

	remote, ok := man.Remotes["caddy"]
	require.True(t, ok, "remote 'caddy' not in manifest")
	require.NotNil(t, remote.OciSource,
		"OciSource nil after unmarshal — koanf struct tags missing on RemoteComponentConfig")

	// Step 2: same as SyncRemotes line 89
	src, err := oci.NewOCISource(remote.OciSource)
	require.NoError(t, err)
	require.NotNil(t, src)

	// NewOCISource calls ParseURL which populates Config's exported fields
	assert.Equal(t, "git.example.com", remote.OciSource.Registry)
	assert.Equal(t, "user/materia-caddy", remote.OciSource.Repository)
	assert.Equal(t, "2026-03-06", remote.OciSource.Tag)
}
