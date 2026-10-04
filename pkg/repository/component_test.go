package repository

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

func Test_InstallComponent(t *testing.T) {
	comp, err := components.NewComponent("test")
	require.NoError(t, comp.Resources.Add(components.Resource{
		Path:     "foo.txt",
		Parent:   "test",
		Kind:     components.ResourceTypeFile,
		Template: false,
		Content:  "hi",
	}))
	require.NoError(t, err)
	fix := newHostFixture(t)
	require.NoError(t, fix.repo.InstallComponent(comp))
	require.True(t, fileExists(t, fix.Data("test", ".component_version")))
	require.True(t, contentIs(t, fix.Data("test", ".component_version"), "Version = 0"), "Got %v", getContent(t, fix.Data("test", ".component_version")))
	require.True(t, fileExists(t, fix.Quad("test", ".materia_managed")))
	require.True(t, contentIs(t, fix.Quad("test", ".materia_managed"), ""))
}

func Test_GetComponent(t *testing.T) {
	fix := newHostFixture(t)
	c := fix.install(t, "test", components.DefaultComponentVersion)
	fix.put(t, c, "hello.container", "[Container]\nImage=foo\n")
	fix.put(t, c, "conf/conf.txt", "")
	fix.put(t, c, "hello.container.d/10-foo.conf", "[Container]\n")

	comp, err := fix.repo.GetComponent("test")
	require.NoError(t, err)
	res, err := comp.Resources.Get("hello.container")
	require.NoError(t, err)
	require.Equal(t, components.ResourceTypeContainer, res.Kind)
	require.Equal(t, "systemd-hello", res.HostObject)

	res, err = comp.Resources.Get("conf")
	require.NoError(t, err)
	require.Equal(t, components.ResourceTypeDirectory, res.Kind)
	require.Equal(t, "", res.HostObject)

	res, err = comp.Resources.Get("conf/conf.txt")
	require.NoError(t, err)
	require.Equal(t, components.ResourceTypeFile, res.Kind)
	require.Equal(t, "", res.HostObject)

	res, err = comp.Resources.Get("hello.container.d")
	require.NoError(t, err)
	require.Equal(t, components.ResourceTypeDropinDir, res.Kind)

	res, err = comp.Resources.Get("hello.container.d/10-foo.conf")
	require.NoError(t, err)
	require.Equal(t, components.ResourceTypeDropin, res.Kind)

	require.Equal(t, 6, comp.Resources.Size())
}

func Test_GetComponent_Corrupt(t *testing.T) {
	fix := newHostFixture(t)
	_ = fix.install(t, "test", components.DefaultComponentVersion)
	require.NoError(t, os.Remove(fix.Data("test", manifests.MateriaManifestFile)))

	_, err := fix.repo.GetComponent("test")
	require.ErrorIs(t, err, components.ErrCorruptComponent)
}

func Test_RemoveComponent(t *testing.T) {
	fix := newHostFixture(t)
	c := fix.install(t, "test", components.DefaultComponentVersion)
	fix.put(t, c, "conf/", "")
	fix.put(t, c, "conf/foo/", "")
	fix.put(t, c, "hello.container.d/", "[Container]\n")
	fix.put(t, c, ".foo.0123456789ab.materia_tmp", "")
	require.NoError(t, os.Remove(fix.Data("test", manifests.MateriaManifestFile)))

	require.NoError(t, fix.repo.RemoveComponent(c))
}

func Test_RemoveComponent_Fail(t *testing.T) {
	fix := newHostFixture(t)
	c := fix.install(t, "test", components.DefaultComponentVersion)
	fix.put(t, c, "conf.txt", "")

	require.Error(t, fix.repo.RemoveComponent(c))
	_, err := fix.repo.GetComponent("test")
	require.NoError(t, err)
}

type hostFixture struct {
	repo          *HostComponentRepository
	data, quadlet string
}

func newHostFixture(t *testing.T) *hostFixture {
	t.Helper()
	base := t.TempDir()
	d, q := filepath.Join(base, "data", "components"), filepath.Join(base, "quadlets")
	repo, err := NewHostComponentRepository(q, d)
	require.NoError(t, err)
	return &hostFixture{repo, d, q}
}

func (h *hostFixture) install(t *testing.T, name string, version int) *components.Component {
	t.Helper()
	c, err := components.NewComponent(name)
	require.NoError(t, err)
	c.Version = version
	require.NoError(t, h.repo.InstallComponent(c))
	require.NoError(t, h.repo.InstallResource(components.Resource{
		Parent: c.InstanceName(), Path: manifests.ComponentManifestFile, Kind: components.ResourceTypeManifest,
	}, nil))
	return c
}

func (h *hostFixture) put(t *testing.T, c *components.Component, path, content string) {
	t.Helper()
	res := components.Resource{Parent: c.InstanceName(), Path: path, Kind: components.FindResourceType(path)}
	require.NoError(t, h.repo.InstallResource(res, []byte(content)))
}

func (h *hostFixture) Data(name, path string) string {
	return filepath.Join(h.data, name, path)
}

func (h *hostFixture) Quad(name, path string) string {
	return filepath.Join(h.quadlet, name, path)
}

func fileExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err != nil || errors.Is(err, fs.ErrNotExist) {
		return false
	}
	return true
}

func contentIs(t *testing.T, path, content string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.TrimSpace(content) == strings.TrimSpace(string(data))
}

func getContent(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
