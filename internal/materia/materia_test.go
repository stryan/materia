package materia

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"primamateria.systems/materia/internal/manifests"
)

// func testMateria(t *testing.T, services []string) *Materia {
// 	var err error
//
// 	log.Debug("loading manifest")
// 	sc := age.Config{
// 		IdentPath: "./test-key.txt",
// 		BaseDir:   "secrets",
// 	}
// 	attributesEngine, err := age.NewAgeStore(sc, sourcedir)
// 	if err != nil {
// 		log.Fatal(fmt.Errorf("error creating age store: %w", err))
// 	}
// 	log.Debug("loading facts")
// 	scripts, err := repository.NewFileRepository(scriptdir)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	servicesrepo, err := repository.NewFileRepository(servicedir)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	sourceRepo, err := repository.NewSourceComponentRepository(sourcedir)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	compRepo, err := repository.NewHostComponentRepository(installdir, filepath.Join(prefix, "components"))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	hm := NewMockHostManager(t)
// 	sm := NewMockSourceManager(t)
// 	m, err := NewMateria(ctx, cfg, hm, attributesEngine, nil, nil, scripts, servicesrepo, sourceRepo, compRepo, sm)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	return m
// }
//
// func TestMain(m *testing.M) {
// 	testPrefix := fmt.Sprintf("/tmp/materia-test-%v", time.Now().Unix())
// 	prefix = filepath.Join(testPrefix, "materia")
// 	installdir = filepath.Join(testPrefix, "install")
// 	servicedir = filepath.Join(testPrefix, "services")
// 	scriptdir = filepath.Join(testPrefix, "scripts")
// 	sourcedir = filepath.Join(testPrefix, "materia", "source")
// 	outputdir = filepath.Join(testPrefix, "materia", "output")
// 	log.Default().SetLevel(log.DebugLevel)
// 	log.Default().SetReportCaller(true)
// 	cfg = &MateriaConfig{
// 		Debug:      true,
// 		Hostname:   "localhost",
// 		Timeout:    0,
// 		Attributes: "age",
// 		MateriaDir: testPrefix,
// 		QuadletDir: installdir,
// 		ServiceDir: servicedir,
// 		ScriptsDir: scriptdir,
// 		SourceDir:  sourcedir,
// 		OutputDir:  outputdir,
// 		User:       &user.User{Uid: "100", Gid: "100", Username: "nonroot", HomeDir: ""},
// 	}
// 	err := os.Mkdir(testPrefix, 0o755)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = os.Mkdir(prefix, 0o755)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = os.Mkdir(sourcedir, 0o755)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = os.Mkdir(installdir, 0o755)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = os.Mkdir(servicedir, 0o755)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = os.Mkdir(scriptdir, 0o755)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	ctx = context.Background()
//
// 	code := m.Run()
// 	_ = os.RemoveAll(testPrefix)
// 	os.Exit(code)
// }
//
// func TestFacts(t *testing.T) {
// 	m := testMateria(t, []string{})
// 	assert.NotNil(t, m.Manifest)
// 	assert.Equal(t, m.Host.GetHostname(), "localhost")
// 	assert.Equal(t, m.Roles, []string(nil))
// 	assigned, err := m.GetAssignedComponents()
// 	assert.NoError(t, err)
// 	assert.Equal(t, assigned, []string{"double", "hello"})
// }

func TestBasic(t *testing.T) {
	hm := NewMockHostManager(t)
	sm := NewMockSourceManager(t)
	sm.EXPECT().LoadManifest(manifests.MateriaManifestFile).Return(&manifests.MateriaManifest{}, nil)
	hm.EXPECT().GetHostname().Return("localhost")
	m, err := NewMateriaFromConfig(context.TODO(), &MateriaConfig{
		QuadletDir: "/tmp/materia/quadlets",
		MateriaDir: "/tmp/materia",
		ServiceDir: "/tmp/services",
		ScriptsDir: "/usr/local/bin",
		SourceDir:  "/materia/source",
	}, hm, sm)
	assert.NoError(t, err)
	assert.NotNil(t, m)
}

// var expectedActions = []Action{
// 	planHelper(ActionInstall, "double", ""),
// 	planHelper(ActionInstall, "double", "inner"),
// 	planHelper(ActionInstall, "double", "goodbye.container"),
// 	planHelper(ActionInstall, "double", "hello.container"),
// 	planHelper(ActionInstall, "double", "hello.timer"),
// 	planHelper(ActionInstall, "double", "inner/test.data"),
// 	planHelper(ActionInstall, "double", "foo"),
// 	planHelper(ActionInstall, "double", manifests.ComponentManifestFile),
// 	planHelper(ActionInstall, "hello", ""),
// 	planHelper(ActionInstall, "hello", "hello.container"),
// 	planHelper(ActionInstall, "hello", "hello.env"),
// 	planHelper(ActionInstall, "hello", "hello.volume"),
// 	planHelper(ActionInstall, "hello", "test.env"),
// 	planHelper(ActionInstall, "hello", manifests.ComponentManifestFile),
// 	planHelper(ActionReload, "", ""),
// 	planHelper(ActionStart, "double", "goodbye.service"),
// 	planHelper(ActionEnable, "double", "hello.timer"),
// 	planHelper(ActionStart, "double", "hello.timer"),
// }
//
// func TestPlan(t *testing.T) {
// 	m := testMateria(t, []string{"hello.service", "double.service", "goodbye.service"})
//
// 	expectedManifest := &manifests.MateriaManifest{
// 		Hosts: map[string]manifests.Host{},
// 	}
// 	expectedManifest.Hosts["localhost"] = manifests.Host{
// 		Components: []string{"hello", "double"},
// 	}
// 	assert.Equal(t, expectedManifest.Hosts, m.Manifest.Hosts)
//
// 	plan, err := m.Plan(ctx)
// 	require.Nil(t, err, "error generating plan")
// 	require.False(t, plan.Empty(), "plan should not be empty")
// 	require.Equal(t, len(plan.Steps()), len(expectedActions), "Length of plan (%v) is not as expected (%v)", len(plan.Steps()), len(expectedActions))
// 	require.Nil(t, plan.Validate(), "generated invalid plan")
//
// 	expectedPlan := NewPlan([]string{}, []string{})
// 	for _, e := range expectedActions {
// 		expectedPlan.Add(e)
// 	}
//
// 	log.Info(plan.Pretty())
// 	log.Info(expectedPlan.Pretty())
// 	for k, v := range plan.Steps() {
// 		expected := expectedPlan.Steps()[k]
// 		if expected.Todo != v.Todo {
// 			t.Fatalf("failed on step %v: expected todo %v != planned %v", k, expected.Todo, v.Todo)
// 		}
// 		if expected.Parent.Name != v.Parent.Name {
// 			t.Fatalf("failed on step %v:expected parent %v != planned  %v", k, expected.Parent.Name, v.Parent.Name)
// 		}
// 		if expected.Target.Path != v.Target.Path {
// 			t.Fatalf("failed on step %v:expected payload %v != planned %v", k, expected.Target.Path, v.Target.Path)
// 		}
// 	}
// }
//
// func TestExecuteFresh(t *testing.T) {
// 	m := testMateria(t, []string{"hello.service", "double.service", "goodbye.service", "hello.timer"})
// 	expectedManifest := &manifests.MateriaManifest{
// 		Hosts: map[string]manifests.Host{},
// 	}
// 	expectedManifest.Hosts["localhost"] = manifests.Host{
// 		Components: []string{"hello", "double"},
// 	}
// 	assert.Equal(t, expectedManifest.Hosts, m.Manifest.Hosts)
//
// 	plan, err := m.Plan(ctx)
// 	require.Nil(t, err)
// 	require.False(t, plan.Empty(), "plan should not be empty")
// 	require.Equal(t, len(plan.Steps()), len(expectedActions), "Length of plan (%v) is not as expected (%v)", len(plan.Steps()), len(expectedActions))
// 	for k, v := range plan.Steps() {
// 		expected := expectedActions[k]
// 		if expected.Todo != v.Todo {
// 			t.Fatalf("failed on step %v: expected todo %v != planned %v", k, expected.Todo, v.Todo)
// 		}
// 		if expected.Parent.Name != v.Parent.Name {
// 			t.Fatalf("failed on step %v:expected parent %v != planned  %v", k, expected.Parent.Name, v.Parent.Name)
// 		}
// 		if expected.Target.Path != v.Target.Path {
// 			t.Fatalf("failed on step %v:expected payload %v != planned %v", k, expected.Target.Path, v.Target.Path)
// 		}
// 	}
// 	count, err := m.Execute(ctx, plan)
// 	assert.Nil(t, err, fmt.Sprintf("error executing plan: %v", err))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	assert.Equal(t, len(plan.Steps()), count, "not every step completed")
// 	// verify all the files are in place
// 	for _, v := range plan.Steps() {
// 		switch v.Todo {
// 		case ActionInstall:
// 			if v.Target.Kind == components.ResourceTypeFile || v.Target.IsQuadlet() {
// 				var dest string
// 				if v.Target.Kind == components.ResourceTypeFile || v.Target.Kind == components.ResourceTypeManifest {
// 					dest = filepath.Join(prefix, "components", v.Parent.Name, v.Target.Path)
// 				} else {
// 					dest = filepath.Join(installdir, v.Parent.Name, v.Target.Path)
// 				}
// 				_, err := os.Stat(dest)
// 				assert.Nil(t, err, fmt.Sprintf("error file not found: %v", v.Target.Path))
// 			} else if v.Target.Kind == components.ResourceTypeComponent {
// 				_, err := os.Stat(fmt.Sprintf("%v/components/%v", prefix, v.Parent.Name))
// 				assert.Nil(t, err, fmt.Sprintf("error component not found: %v", v.Target.Path))
// 				_, err = os.Stat(fmt.Sprintf("%v/%v", installdir, v.Parent.Name))
// 				assert.Nil(t, err, fmt.Sprintf("error component not found: %v", v.Target.Path))
// 			}
// 		case ActionStart:
// 			if v.Target.Kind == components.ResourceTypeService {
// 				state, err := m.Host.Get(ctx, v.Target.Path)
// 				assert.Nil(t, err, "error getting service state")
// 				assert.Equal(t, "active", state.State)
// 			}
// 		}
// 	}
// }
//
// func planHelper(todo ActionType, name, res string) Action {
// 	if res == "" {
// 		if name == "" {
// 			return Action{
// 				Todo: ActionReload,
// 				Parent: &components.Component{
// 					Name: "root",
// 				},
// 				Target: components.Resource{
// 					Parent: name,
// 					Kind:   components.ResourceTypeHost,
// 				},
// 			}
// 		} else {
// 			return Action{
// 				Todo:   todo,
// 				Parent: &components.Component{Name: name},
// 				Target: components.Resource{
// 					Parent: name,
// 					Kind:   components.ResourceTypeComponent,
// 					Path:   name,
// 				},
// 			}
// 		}
// 	}
// 	act := Action{
// 		Todo: todo,
// 		Parent: &components.Component{
// 			Name: name,
// 		},
// 		Target: components.Resource{
// 			Parent: name,
// 			Kind:   components.FindResourceType(res),
// 			Path:   res,
// 		},
// 	}
// 	return act
// }
