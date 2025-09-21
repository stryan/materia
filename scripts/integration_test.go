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
	mockcontainers := &MockContainers{make(map[string]string), make(map[string]string)}
	for _, v := range services {
		mockservices.Services[v] = "unknown"
	}

	log.Debug("updating configured source cache")
	err = source.Sync(ctx)
	if err != nil {
		log.Fatalf("error syncing source: %v", err)
	}
	log.Debug("loading manifest")
	man, err := manifests.LoadMateriaManifest(filepath.Join(cfg.SourceDir, manifests.MateriaManifestFile))
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
	// _ = os.RemoveAll(testPrefix)
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
	planHelper(materia.ActionInstall, "double", ""),
	planHelper(materia.ActionInstall, "double", "inner"),
	planHelper(materia.ActionInstall, "double", "goodbye.container"),
	planHelper(materia.ActionInstall, "double", "hello.container"),
	planHelper(materia.ActionInstall, "double", "hello.timer"),
	planHelper(materia.ActionInstall, "double", "inner/test.data"),
	planHelper(materia.ActionInstall, "double", "foo"),
	planHelper(materia.ActionInstall, "double", manifests.ComponentManifestFile),
	planHelper(materia.ActionInstall, "hello", ""),
	planHelper(materia.ActionInstall, "hello", "hello.container"),
	planHelper(materia.ActionInstall, "hello", "hello.env"),
	planHelper(materia.ActionInstall, "hello", "hello.volume"),
	planHelper(materia.ActionInstall, "hello", "test.env"),
	planHelper(materia.ActionInstall, "hello", manifests.ComponentManifestFile),
	planHelper(materia.ActionReload, "", ""),
	planHelper(materia.ActionStart, "double", "goodbye.service"),
	planHelper(materia.ActionEnable, "double", "hello.timer"),
	planHelper(materia.ActionStart, "double", "hello.timer"),
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
	require.Nil(t, err, "error generating plan")
	require.False(t, plan.Empty(), "plan should not be empty")
	require.Equal(t, len(plan.Steps()), len(expectedActions), "Length of plan (%v) is not as expected (%v)", len(plan.Steps()), len(expectedActions))
	require.Nil(t, plan.Validate(), "generated invalid plan")

	expectedPlan := materia.NewPlan(m.InstalledComponents, []string{})
	for _, e := range expectedActions {
		expectedPlan.Add(e)
	}

	log.Info(plan.Pretty())
	log.Info(expectedPlan.Pretty())
	for k, v := range plan.Steps() {
		expected := expectedPlan.Steps()[k]
		if expected.Todo != v.Todo {
			t.Fatalf("failed on step %v: expected todo %v != planned %v", k, expected.Todo, v.Todo)
		}
		if expected.Parent.Name != v.Parent.Name {
			t.Fatalf("failed on step %v:expected parent %v != planned  %v", k, expected.Parent.Name, v.Parent.Name)
		}
		if expected.Payload.Path != v.Payload.Path {
			t.Fatalf("failed on step %v:expected payload %v != planned %v", k, expected.Payload.Path, v.Payload.Path)
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
		if expected.Payload.Path != v.Payload.Path {
			t.Fatalf("failed on step %v:expected payload %v != planned %v", k, expected.Payload.Path, v.Payload.Path)
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
		case materia.ActionInstall:
			if v.Payload.Kind == components.ResourceTypeFile || v.Payload.IsQuadlet() {
				var dest string
				if v.Payload.Kind == components.ResourceTypeFile || v.Payload.Kind == components.ResourceTypeManifest {
					dest = filepath.Join(prefix, "components", v.Parent.Name, v.Payload.Path)
				} else {
					dest = filepath.Join(installdir, v.Parent.Name, v.Payload.Path)
				}
				_, err := os.Stat(dest)
				assert.Nil(t, err, fmt.Sprintf("error file not found: %v", v.Payload.Path))
			} else if v.Payload.Kind == components.ResourceTypeComponent {
				_, err := os.Stat(fmt.Sprintf("%v/components/%v", prefix, v.Parent.Name))
				assert.Nil(t, err, fmt.Sprintf("error component not found: %v", v.Payload.Path))
				_, err = os.Stat(fmt.Sprintf("%v/%v", installdir, v.Parent.Name))
				assert.Nil(t, err, fmt.Sprintf("error component not found: %v", v.Payload.Path))
			}
		case materia.ActionStart:
			if v.Payload.Kind == components.ResourceTypeService {
				state, err := m.Services.Get(ctx, v.Payload.Path)
				assert.Nil(t, err, "error getting service state")
				assert.Equal(t, "active", state.State)
			}
		}
	}
}

func planHelper(todo materia.ActionType, name, res string) materia.Action {
	if res == "" {
		if name == "" {
			return materia.Action{
				Todo: materia.ActionReload,
				Parent: &components.Component{
					Name: "root",
				},
				Payload: components.Resource{
					Parent: name,
					Kind:   components.ResourceTypeHost,
				},
			}
		} else {
			return materia.Action{
				Todo:   todo,
				Parent: &components.Component{Name: name},
				Payload: components.Resource{
					Parent: name,
					Kind:   components.ResourceTypeComponent,
					Path:   name,
				},
			}
		}
	}
	act := materia.Action{
		Todo: todo,
		Parent: &components.Component{
			Name: name,
		},
		Payload: components.Resource{
			Parent: name,
			Kind:   components.FindResourceType(res),
			Path:   res,
		},
	}
	return act
}
