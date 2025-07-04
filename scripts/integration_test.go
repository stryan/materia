package main

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/internal/components"
	fprov "primamateria.systems/materia/internal/facts"
	"primamateria.systems/materia/internal/manifests"
	"primamateria.systems/materia/internal/materia"
	"primamateria.systems/materia/internal/repository"
	"primamateria.systems/materia/internal/secrets/age"
	"primamateria.systems/materia/internal/source"

	filesource "primamateria.systems/materia/internal/source/file"
)

var (
	ctx                                                             context.Context
	cfg                                                             *materia.MateriaConfig
	prefix, installdir, servicedir, scriptdir, sourcedir, outputdir string
)

func testMateria(services []string) *materia.Materia {
	var source source.Source
	var err error

	source = &filesource.FileSource{
		RemoteRepository: "./testrepo",
		Destination:      sourcedir,
	}

	mockservices := &MockServices{}
	mockservices.Services = make(map[string]string)
	mockcontainers := &MockContainers{make(map[string]string)}
	for _, v := range services {
		mockservices.Services[v] = "unknown"
	}

	log.Debug("updating configured source cache")
	err = source.Sync(ctx)
	if err != nil {
		log.Fatalf("error syncing source: %v", err)
	}
	log.Debug("loading manifest")
	man, err := manifests.LoadMateriaManifest(filepath.Join(cfg.SourceDir, "MANIFEST.toml"))
	if err != nil {
		log.Fatalf("error loading manifest: %v", err)
	}
	if err := man.Validate(); err != nil {
		log.Fatal(err)
	}
	sc := age.Config{
		IdentPath: "./test-key.txt",
		BaseDir:   "secrets",
	}
	secretManager, err := age.NewAgeStore(sc, sourcedir)
	if err != nil {
		log.Fatal(fmt.Errorf("error creating age store: %w", err))
	}
	log.Debug("loading facts")
	facts, err := fprov.NewHostFacts(ctx, cfg.Hostname)
	if err != nil {
		log.Fatalf("error generating facts: %v", err)
	}
	scripts, err := repository.NewFileRepository(scriptdir)
	if err != nil {
		log.Fatal(err)
	}
	servicesrepo, err := repository.NewFileRepository(servicedir)
	if err != nil {
		log.Fatal(err)
	}
	sourceRepo, err := repository.NewSourceComponentRepository(filepath.Join(sourcedir, "components"))
	if err != nil {
		log.Fatal(err)
	}
	compRepo, err := repository.NewHostComponentRepository(installdir, filepath.Join(prefix, "components"))
	if err != nil {
		log.Fatal(err)
	}
	m, err := materia.NewMateria(ctx, cfg, source, man, facts, secretManager, mockservices, mockcontainers, scripts, servicesrepo, sourceRepo, compRepo)
	if err != nil {
		log.Fatal(err)
	}
	return m
}

func TestMain(m *testing.M) {
	testPrefix := fmt.Sprintf("/tmp/materia-test-%v", time.Now().Unix())
	prefix = filepath.Join(testPrefix, "materia")
	installdir = filepath.Join(testPrefix, "install")
	servicedir = filepath.Join(testPrefix, "services")
	scriptdir = filepath.Join(testPrefix, "scripts")
	sourcedir = filepath.Join(testPrefix, "materia", "source")
	outputdir = filepath.Join(testPrefix, "materia", "output")
	log.Default().SetLevel(log.DebugLevel)
	log.Default().SetReportCaller(true)
	cfg = &materia.MateriaConfig{
		Debug:      true,
		Hostname:   "localhost",
		Timeout:    0,
		MateriaDir: testPrefix,
		QuadletDir: installdir,
		ServiceDir: servicedir,
		ScriptsDir: scriptdir,
		SourceDir:  sourcedir,
		OutputDir:  outputdir,
		User:       &user.User{Uid: "100", Gid: "100", Username: "nonroot", HomeDir: ""},
	}
	err := os.Mkdir(testPrefix, 0o755)
	if err != nil {
		log.Fatal(err)
	}
	err = os.Mkdir(prefix, 0o755)
	if err != nil {
		log.Fatal(err)
	}
	err = os.Mkdir(sourcedir, 0o755)
	if err != nil {
		log.Fatal(err)
	}
	err = os.Mkdir(installdir, 0o755)
	if err != nil {
		log.Fatal(err)
	}
	err = os.Mkdir(servicedir, 0o755)
	if err != nil {
		log.Fatal(err)
	}
	err = os.Mkdir(scriptdir, 0o755)
	if err != nil {
		log.Fatal(err)
	}
	ctx = context.Background()

	code := m.Run()
	_ = os.RemoveAll(testPrefix)
	os.Exit(code)
}

func TestFacts(t *testing.T) {
	m := testMateria([]string{})
	assert.NotNil(t, m.Manifest)
	assert.Equal(t, m.HostFacts.GetHostname(), "localhost")
	assert.Equal(t, m.Roles, []string(nil))
	assert.Equal(t, m.AssignedComponents, []string{"double", "hello"})
	assert.Equal(t, m.InstalledComponents, []string(nil))
}

var expectedActions = []materia.Action{
	planHelper(materia.ActionInstallComponent, "double", "", ""),
	planHelper(materia.ActionInstallQuadlet, "double", "goodbye.container", ""),
	planHelper(materia.ActionInstallQuadlet, "double", "hello.container", ""),
	planHelper(materia.ActionInstallService, "double", "hello.timer", ""),
	planHelper(materia.ActionInstallFile, "double", "test.data", "/inner/"),
	planHelper(materia.ActionInstallFile, "double", "MANIFEST.toml", ""),
	planHelper(materia.ActionInstallComponent, "hello", "", ""),
	planHelper(materia.ActionInstallQuadlet, "hello", "hello.container", ""),
	planHelper(materia.ActionInstallFile, "hello", "hello.env", ""),
	planHelper(materia.ActionInstallQuadlet, "hello", "hello.volume", ""),
	planHelper(materia.ActionInstallFile, "hello", "test.env", ""),
	planHelper(materia.ActionInstallFile, "hello", "MANIFEST.toml", ""),
	planHelper(materia.ActionReloadUnits, "root", "", ""),
	planHelper(materia.ActionStartService, "double", "goodbye.service", ""),
	planHelper(materia.ActionEnableService, "double", "hello.timer", ""),
	planHelper(materia.ActionStartService, "double", "hello.timer", ""),
}

func TestPlan(t *testing.T) {
	m := testMateria([]string{"hello.service", "double.service", "goodbye.service"})

	expectedManifest := &manifests.MateriaManifest{
		Secrets: "age",
		Hosts:   map[string]manifests.Host{},
	}
	expectedManifest.Hosts["localhost"] = manifests.Host{
		Components: []string{"hello", "double"},
	}
	assert.Equal(t, expectedManifest.Hosts, m.Manifest.Hosts)
	assert.Equal(t, expectedManifest.Secrets, m.Manifest.Secrets)
	plan, err := m.Plan(ctx)
	require.Nil(t, err)
	require.False(t, plan.Empty(), "plan should not be empty")
	require.Equal(t, len(plan.Steps()), len(expectedActions), "Length of plan (%v) is not as expected (%v)", len(plan.Steps()), len(expectedActions))
	require.Nil(t, plan.Validate(), "generated invalid plan")

	expectedPlan := materia.NewPlan(m.InstalledComponents, []string{})
	for _, e := range expectedActions {
		expectedPlan.Add(e)
	}

	log.Info(expectedPlan.Pretty())
	for k, v := range plan.Steps() {
		expected := expectedPlan.Steps()[k]
		if expected.Todo != v.Todo {
			t.Fatalf("failed on step %v: expected todo %v != planned %v", k, expected.Todo, v.Todo)
		}
		if expected.Parent.Name != v.Parent.Name {
			t.Fatalf("failed on step %v:expected parent %v != planned  %v", k, expected.Parent.Name, v.Parent.Name)
		}
		if expected.Payload.Name != v.Payload.Name {
			t.Fatalf("failed on step %v:expected payload %v != planned %v", k, expected.Payload.Name, v.Payload.Name)
		}
	}
}

func TestExecuteFresh(t *testing.T) {
	m := testMateria([]string{"hello.service", "double.service", "goodbye.service", "hello.timer"})
	expectedManifest := &manifests.MateriaManifest{
		Secrets: "age",
		Hosts:   map[string]manifests.Host{},
	}
	expectedManifest.Hosts["localhost"] = manifests.Host{
		Components: []string{"hello", "double"},
	}
	assert.Equal(t, expectedManifest.Hosts, m.Manifest.Hosts)
	assert.Equal(t, expectedManifest.Secrets, m.Manifest.Secrets)
	plan, err := m.Plan(ctx)
	require.Nil(t, err)
	require.False(t, plan.Empty(), "plan should not be empty")
	require.Equal(t, len(plan.Steps()), len(expectedActions), "Length of plan (%v) is not as expected (%v)", len(plan.Steps()), len(expectedActions))
	for k, v := range plan.Steps() {
		expected := expectedActions[k]
		if expected.Todo != v.Todo {
			t.Fatalf("failed on step %v: expected todo %v != planned %v", k, expected.Todo, v.Todo)
		}
		if expected.Parent.Name != v.Parent.Name {
			t.Fatalf("failed on step %v:expected parent %v != planned  %v", k, expected.Parent.Name, v.Parent.Name)
		}
		if expected.Payload.Name != v.Payload.Name {
			t.Fatalf("failed on step %v:expected payload %v != planned %v", k, expected.Payload.Name, v.Payload.Name)
		}
	}
	count, err := m.Execute(ctx, plan)
	assert.Nil(t, err, fmt.Sprintf("error executing plan: %v", err))
	if err != nil {
		log.Fatal(err)
	}
	assert.Equal(t, len(plan.Steps()), count, "not every step completed")
	// verify all the files are in place
	for _, v := range plan.Steps() {
		switch v.Todo {
		case materia.ActionInstallComponent:
			_, err := os.Stat(fmt.Sprintf("%v/components/%v", prefix, v.Parent.Name))
			assert.Nil(t, err, fmt.Sprintf("error component not found: %v", v.Payload.Name))
			_, err = os.Stat(fmt.Sprintf("%v/%v", installdir, v.Parent.Name))
			assert.Nil(t, err, fmt.Sprintf("error component not found: %v", v.Payload.Name))

		case materia.ActionInstallFile, materia.ActionInstallQuadlet:
			var dest string
			if v.Payload.Kind == components.ResourceTypeFile || v.Payload.Kind == components.ResourceTypeManifest {
				dir := filepath.Dir(v.Payload.Path)
				dest = filepath.Join(prefix, "components", v.Parent.Name, dir, v.Payload.Name)
			} else {
				dest = filepath.Join(installdir, v.Parent.Name, v.Payload.Name)
			}
			_, err := os.Stat(dest)
			assert.Nil(t, err, fmt.Sprintf("error file not found: %v", v.Payload.Name))
		case materia.ActionStartService:
			state, err := m.Services.Get(ctx, v.Payload.Name)
			assert.Nil(t, err, "error getting service state")
			assert.Equal(t, "active", state.State)
		}
	}
}

func planHelper(todo materia.ActionType, name, res, parentPath string) materia.Action {
	if parentPath == "" {
		parentPath = "/"
	}
	act := materia.Action{
		Todo: todo,
		Parent: &components.Component{
			Name: name,
		},
		Payload: components.Resource{
			Parent: name,
			Name:   res,
			Path:   filepath.Join(parentPath, res),
		},
	}
	return act
}
