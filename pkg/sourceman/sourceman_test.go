package sourceman

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/internal/source/oci"
	"primamateria.systems/materia/pkg/manifests"
)

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
