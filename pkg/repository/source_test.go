package repository

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

func Test_SourceRepository_GetLocal(t *testing.T) {
	fix := newSrcFixture(t)
	_ = fix.component(t, "test", map[string]string{
		"hello.container": "[Container]\nImage=foo",
		"conf/conf.txt":   "config",
	})
	comp, err := fix.repo.GetComponent("test")
	require.NoError(t, err)

	res, err := comp.Resources.Get("hello.container")
	require.NoError(t, err)
	require.Equal(t, components.ResourceTypeContainer, res.Kind)

	res, err = comp.Resources.Get("conf/conf.txt")
	require.NoError(t, err)
	require.Equal(t, components.ResourceTypeFile, res.Kind)
}

func Test_SourceRepository_GetRemote(t *testing.T) {
	fix := newSrcFixture(t)
	_ = fix.remote(t, "remtest", "")
	require.NoError(t, fix.reg.Register("remtest", ""))
	comp, err := fix.repo.GetComponent("remtest")
	require.NoError(t, err)
	require.Equal(t, 1, comp.Resources.Size())
}

func Test_SourceRepository_ListComponentNames(t *testing.T) {
	fix := newSrcFixture(t)
	fix.component(t, "test", map[string]string{
		"hello.container": "[Container]\nImage=foo",
		"conf/conf.txt":   "config",
	})
	_ = fix.remote(t, "remtest", "")
	require.NoError(t, fix.reg.Register("remtest", ""))

	_ = fix.remote(t, "test", "")
	require.NoError(t, fix.reg.Register("remtest", ""))

	names, err := fix.repo.ListComponentNames()
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"test", "remtest"}, names)
	comp, err := fix.repo.GetComponent("test")
	require.NoError(t, err)

	res, err := comp.Resources.Get("hello.container")
	require.NoError(t, err)
	require.Equal(t, components.ResourceTypeContainer, res.Kind)
}

type srcFixture struct {
	repo      *SourceComponentRepository
	reg       *RemoteComponentRegistry
	localDir  string
	remoteDir string
}

func newSrcFixture(t *testing.T) *srcFixture {
	t.Helper()
	base := t.TempDir()
	dir, remote := filepath.Join(base, "source"), filepath.Join(base, "remote")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "components"), 0o755))
	reg, err := NewRemoteComponentRegistry(remote)
	require.NoError(t, err)
	repo, err := NewSourceComponentRepository(dir, reg)
	require.NoError(t, err)
	return &srcFixture{repo, reg, dir, remote}
}

func (f *srcFixture) component(t *testing.T, name string, files map[string]string) string {
	t.Helper()
	cdir := filepath.Join(f.localDir, "components", name)
	writeFile(t, cdir, manifests.ComponentManifestFile, "", 0o644)
	for rel, content := range files {
		writeFile(t, cdir, rel, content, 0o644)
	}
	return cdir
}

func (f *srcFixture) remote(t *testing.T, name, subpath string) string {
	t.Helper()
	clone, err := f.reg.ClonePath(name)
	require.NoError(t, err)
	writeFile(t, filepath.Join(clone, subpath), manifests.ComponentManifestFile, "", 0o644)
	return clone
}
