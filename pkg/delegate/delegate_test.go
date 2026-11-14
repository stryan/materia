package delegate_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/attributes"
	fileattrs "primamateria.systems/materia/pkg/attributes/file"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/delegate"
	"primamateria.systems/materia/pkg/loader"
	"primamateria.systems/materia/pkg/macros"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/repository"
)

const webManifest = `Secrets = ["token"]

[Settings]
User = "webapp"
SetupScript = "setup.sh"
CleanupScript = "cleanup.sh"

[Defaults]
port = "8080"

[[Services]]
Service = "web.service"

[SomeOtherTool]
Key = "ignored by materia"
`

const webContainer = `[Container]
Image=localhost/web
Volume={{ m_dataDir "web" }}/conf:/conf
Environment=PORT={{ .port }}
`

func writeTree(t *testing.T, base string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		full := filepath.Join(base, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
}

// dataDirMacros stands in for materia's macros: only the data directory differs per user.
func dataDirMacros(dataDir string) macros.MacroMap {
	return func(map[string]any) template.FuncMap {
		return template.FuncMap{
			"m_dataDir": func(arg string) string { return filepath.Join(dataDir, arg) },
		}
	}
}

func loadSource(t *testing.T, sourceDir, name string, m macros.MacroMap, attrs map[string]any) *components.Component {
	t.Helper()
	repo, err := repository.NewSourceComponentRepository(sourceDir, nil)
	require.NoError(t, err)
	comp, err := components.NewComponent(name)
	require.NoError(t, err)
	require.NoError(t, loader.NewSourceComponentPipeline(repo, m, attrs, nil, nil).Load(context.Background(), comp))
	return comp
}

func resourcePaths(c *components.Component) []string {
	var paths []string
	for _, r := range c.Resources.List() {
		paths = append(paths, r.Path)
	}
	return paths
}

func loadWeb(t *testing.T, source string) *components.Component {
	t.Helper()
	comp := loadSource(t, source, "web", dataDirMacros("/var/lib/materia/components"), map[string]any{"token": "s3cret"})
	require.NotNil(t, comp.Delegation)
	comp.Delegation.Repository = components.DelegatedRepository{
		Hostname: "forge",
		Roles:    []string{"streaming"},
		Snippets: []manifests.SnippetConfig{{Name: "greet", Body: "hello"}},
	}
	return comp
}

func newSource(t *testing.T) string {
	t.Helper()
	source := t.TempDir()
	writeTree(t, source, map[string]string{
		"components/web/MANIFEST.toml":         webManifest,
		"components/web/web.container.gotmpl":  webContainer,
		"components/web/conf/app.conf":         "setting = 1\n",
		"components/web/setup.sh":              "#!/bin/sh\nexit 0\n",
		"components/web/cleanup.sh":            "#!/bin/sh\nexit 0\n",
		"components/plain/MANIFEST.toml":       "",
		"components/plain/plain.container":     "[Container]\nImage=localhost/plain\n",
		"components/plain/setup.sh":            "#!/bin/sh\n",
		"components/instanced/MANIFEST.toml":   "[Settings]\nUser = \"webapp\"\n",
		"components/rooted/MANIFEST.toml":      "[Settings]\nUser = \"root\"\n",
		"components/traversal/MANIFEST.toml":   "[Settings]\nUser = \"../etc\"\n",
		"components/traversal/trav.container":  "[Container]\nImage=x\n",
		"components/instanced/inst@.container": "[Container]\nImage=x\n",
	})
	return source
}

func TestDelegationStage_SplitsComponent(t *testing.T) {
	comp := loadWeb(t, newSource(t))

	assert.Equal(t, "webapp", comp.User)
	// the delegating materia keeps only the record: manifest and lifecycle scripts
	assert.ElementsMatch(t, []string{"MANIFEST.toml", "setup.sh", "cleanup.sh"}, resourcePaths(comp))
	assert.Zero(t, comp.ServiceManifests.Size(), "services belong to the account's materia")

	staged := map[string]components.DelegatedFile{}
	for _, f := range comp.Delegation.Files {
		staged[f.Path] = f
	}
	assert.Len(t, staged, 3)
	assert.Equal(t, webContainer, staged["web.container.gotmpl"].Content, "templates must stay unrendered")
	assert.True(t, staged["conf"].Dir)
	assert.Equal(t, "setting = 1\n", staged["conf/app.conf"].Content)

	manifest := string(comp.Delegation.Manifest)
	for _, absent := range []string{"User", "SetupScript", "CleanupScript", "SomeOtherTool"} {
		assert.NotContains(t, manifest, absent)
	}
	assert.Contains(t, manifest, "web.service")
	assert.Equal(t, map[string]any{"token": "s3cret"}, comp.Delegation.Attributes)
}

func TestDelegationStage_LeavesNormalComponentsAlone(t *testing.T) {
	comp := loadSource(t, newSource(t), "plain", dataDirMacros("/data"), nil)
	assert.Empty(t, comp.User)
	assert.Nil(t, comp.Delegation)
	assert.ElementsMatch(t, []string{"MANIFEST.toml", "plain.container", "setup.sh"}, resourcePaths(comp))
}

func TestDelegationStage_Rejects(t *testing.T) {
	source := newSource(t)
	repo, err := repository.NewSourceComponentRepository(source, nil)
	require.NoError(t, err)
	for _, name := range []string{"instanced@one", "rooted", "traversal"} {
		comp, err := components.NewComponent(name)
		require.NoError(t, err)
		err = loader.NewSourceComponentPipeline(repo, dataDirMacros("/data"), nil, nil, nil).Load(context.Background(), comp)
		assert.Error(t, err, name)
	}
}

func TestDelegationStage_OverrideSetsUser(t *testing.T) {
	source := newSource(t)
	repo, err := repository.NewSourceComponentRepository(source, nil)
	require.NoError(t, err)
	comp, err := components.NewComponent("plain")
	require.NoError(t, err)
	extension := &manifests.ComponentManifest{Settings: manifests.Settings{User: new("webapp")}}
	pipeline := loader.NewSourceComponentPipeline(repo, dataDirMacros("/data"), nil, nil, []*manifests.ComponentManifest{extension})
	require.NoError(t, pipeline.Load(context.Background(), comp))
	assert.Equal(t, "webapp", comp.User)
	assert.ElementsMatch(t, []string{"MANIFEST.toml"}, resourcePaths(comp))
}

// The staged directory must be a repository that an unchanged materia can
// load as another user, and that materia must render with its own paths.
func TestStore_AccountLoadsStagedRepository(t *testing.T) {
	ctx := context.Background()
	// a strict umask must not hide the staged files from the account
	umask := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(umask) })
	store := delegate.NewStore(filepath.Join(t.TempDir(), "delegate"))
	require.NoError(t, store.Stage(loadWeb(t, newSource(t))))

	account := store.AccountDir("webapp")
	info, err := os.Stat(account)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o750), info.Mode().Perm(), "only root may read the staging directory until someone grants the account access")

	// The account gets the group of the top directory and nothing else, so it
	// reads everything below as "other".
	require.NoError(t, filepath.WalkDir(account, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == account {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		want := fs.FileMode(0o004)
		if d.IsDir() {
			want = 0o005
		}
		assert.Equal(t, want, info.Mode().Perm()&want, "the account can't read %v", path)
		return nil
	}))

	repoManifest, err := manifests.LoadMateriaManifest(filepath.Join(account, manifests.MateriaManifestFile))
	require.NoError(t, err)
	require.NoError(t, repoManifest.Validate())
	assert.Equal(t, []string{"web"}, repoManifest.Hosts["all"].Components)
	assert.Equal(t, []string{"streaming"}, repoManifest.Hosts["forge"].Roles)
	assert.Equal(t, "greet", repoManifest.Snippets[0].Name)

	vault, err := fileattrs.NewFileStore(fileattrs.Config{BaseDir: delegate.AttributesDir, LoadAllVaults: true}, account)
	require.NoError(t, err)
	attrs, err := vault.Lookup(ctx, attributes.AttributesFilter{Hostname: "forge", Component: "web"})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"token": "s3cret"}, attrs)

	comp := loadSource(t, account, "web", dataDirMacros("/home/webapp/.local/share/materia/components"), attrs)
	assert.Empty(t, comp.User, "the account's materia must not delegate again")
	assert.Nil(t, comp.Delegation)
	assert.Empty(t, comp.Config.GetSetupScript(), "lifecycle scripts run only in the delegating materia")
	assert.False(t, comp.Resources.Contains("setup.sh"))
	assert.Equal(t, 1, comp.ServiceManifests.Size())

	container, err := comp.Resources.Get("web.container")
	require.NoError(t, err)
	assert.Contains(t, container.Content, "Volume=/home/webapp/.local/share/materia/components/web/conf:/conf")
	assert.Contains(t, container.Content, "PORT=8080")
	secret, err := comp.Resources.Get("token")
	require.NoError(t, err)
	assert.Equal(t, components.ResourceTypePodmanSecret, secret.Kind)
	assert.Equal(t, "s3cret", secret.Content)
}

func TestStore_DeliveryState(t *testing.T) {
	source := newSource(t)
	store := delegate.NewStore(filepath.Join(t.TempDir(), "delegate"))
	comp := loadWeb(t, source)
	first, err := comp.Delegation.Digest()
	require.NoError(t, err)

	_, ok, err := store.Delivered("webapp", "web")
	require.NoError(t, err)
	assert.False(t, ok, "nothing delivered before the first stage")

	require.NoError(t, store.Stage(comp))
	_, ok, err = store.Delivered("webapp", "web")
	require.NoError(t, err)
	assert.False(t, ok, "staging alone is not delivery")

	require.NoError(t, store.MarkDelivered("webapp"))
	got, ok, err := store.Delivered("webapp", "web")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, first, got)

	// a change to any staged file changes the digest and replaces the copy
	writeTree(t, source, map[string]string{"components/web/conf/app.conf": "setting = 2\n"})
	require.NoError(t, os.Remove(filepath.Join(source, "components/web/web.container.gotmpl")))
	changed := loadWeb(t, source)
	second, err := changed.Delegation.Digest()
	require.NoError(t, err)
	assert.NotEqual(t, first, second)
	require.NoError(t, store.Stage(changed))
	staged := filepath.Join(store.AccountDir("webapp"), "components", "web")
	content, err := os.ReadFile(filepath.Join(staged, "conf", "app.conf"))
	require.NoError(t, err)
	assert.Equal(t, "setting = 2\n", string(content))
	assert.NoFileExists(t, filepath.Join(staged, "web.container.gotmpl"), "a file removed from the source must leave the staged copy")
	got, _, err = store.Delivered("webapp", "web")
	require.NoError(t, err)
	assert.Equal(t, first, got, "delivery state changes only after the account applied the new copy")
	entries, err := os.ReadDir(filepath.Dir(staged))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary directories may be left behind")
}

func TestStore_Unstage(t *testing.T) {
	store := delegate.NewStore(filepath.Join(t.TempDir(), "delegate"))
	require.NoError(t, store.Unstage("webapp", "web"), "unstaging what was never staged is fine")

	require.NoError(t, store.Stage(loadWeb(t, newSource(t))))
	require.NoError(t, store.MarkDelivered("webapp"))
	require.NoError(t, store.Unstage("webapp", "web"))

	account := store.AccountDir("webapp")
	assert.NoDirExists(t, filepath.Join(account, "components", "web"))
	assert.NoFileExists(t, filepath.Join(account, delegate.AttributesDir, "web.toml"))
	repoManifest, err := manifests.LoadMateriaManifest(filepath.Join(account, manifests.MateriaManifestFile))
	require.NoError(t, err)
	assert.Empty(t, repoManifest.Hosts["all"].Components)
	assert.Equal(t, []string{"streaming"}, repoManifest.Hosts["forge"].Roles, "host facts survive an unstage")

	_, ok, err := store.Delivered("webapp", "web")
	require.NoError(t, err)
	assert.True(t, ok, "still delivered until the account's materia removed it")
	require.NoError(t, store.MarkDelivered("webapp"))
	_, ok, err = store.Delivered("webapp", "web")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestStore_RejectsBadNames(t *testing.T) {
	store := delegate.NewStore(filepath.Join(t.TempDir(), "delegate"))
	assert.Error(t, store.Unstage("../escape", "web"))
	assert.Error(t, store.Unstage("webapp", "../escape"))
	_, _, err := store.Delivered("root", "web")
	assert.Error(t, err)
}
