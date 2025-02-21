package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"git.saintnet.tech/stryan/materia/internal/materia"
	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	"github.com/stretchr/testify/assert"
)

var (
	ctx                context.Context
	cfg                *materia.Config
	prefix, installdir string
)

func testMateria(services []string) *materia.Materia {
	mockservices := &MockServices{}
	mockservices.Services = make(map[string]string)
	mockcontainers := &MockContainers{make(map[string]string)}
	for _, v := range services {
		mockservices.Services[v] = "unknown"
	}
	m, err := materia.NewMateria(ctx, cfg, mockservices, mockcontainers)
	if err != nil {
		log.Fatal(err)
	}
	return m
}

func TestMain(m *testing.M) {
	testPrefix := fmt.Sprintf("/tmp/materia-test-%v", time.Now().Unix())
	prefix = filepath.Join(testPrefix, "materia")
	installdir = filepath.Join(testPrefix, "install")
	cfg = &materia.Config{
		SourceURL:   "file://./testrepo",
		Debug:       false,
		Hostname:    "localhost",
		Timeout:     0,
		Prefix:      testPrefix,
		Destination: installdir,
		User: &user.User{
			Uid:      "100",
			Gid:      "100",
			Username: "nonroot",
			HomeDir:  "",
		},
	}
	err := os.Mkdir(testPrefix, 0o755)
	if err != nil {
		log.Fatal(err)
	}
	err = os.Mkdir(filepath.Join(testPrefix, "materia"), 0o755)
	if err != nil {
		log.Fatal(err)
	}
	err = os.Mkdir(installdir, 0o755)
	if err != nil {
		log.Fatal(err)
	}

	ctx = context.Background()

	code := m.Run()
	os.RemoveAll(testPrefix)
	os.Exit(code)
}

func TestFacts(t *testing.T) {
	m := testMateria([]string{})
	manifest, facts, err := m.Facts(ctx, cfg)
	assert.Nil(t, err)
	assert.NotNil(t, manifest)
	assert.NotNil(t, facts)
	assert.Equal(t, facts, &materia.Facts{
		Hostname: "localhost",
		Roles:    nil,
	})
}

func TestPlan(t *testing.T) {
	m := testMateria([]string{"hello.service", "double.service", "goodbye.service"})
	manifest, facts, err := m.Facts(ctx, cfg)
	assert.Nil(t, err)
	assert.NotNil(t, manifest)
	assert.NotNil(t, facts)
	assert.Equal(t, facts, &materia.Facts{
		Hostname: "localhost",
		Roles:    nil,
	})
	expectedManifest := &materia.MateriaManifest{
		Secrets: "age",
		Hosts:   map[string]materia.Host{},
	}
	expectedManifest.Hosts["localhost"] = materia.Host{
		Components: []string{"hello", "double"},
	}
	assert.Equal(t, expectedManifest.Hosts, manifest.Hosts)
	assert.Equal(t, expectedManifest.Secrets, manifest.Secrets)
	fixAgeManifest(manifest)
	err = m.Prepare(ctx, manifest)
	assert.Nil(t, err, fmt.Sprintf("error preparing: %v", err))
	plan, err := m.Plan(ctx, manifest, facts)
	assert.Nil(t, err)
	if err != nil {
		t.Fail()
	}
	expectedPlan := []materia.Action{
		planHelper(materia.ActionInstallComponent, "double", ""),
		planHelper(materia.ActionInstallResource, "double", "MANIFEST.toml"),
		planHelper(materia.ActionInstallResource, "double", "goodbye.container"),
		planHelper(materia.ActionInstallResource, "double", "hello.container"),
		planHelper(materia.ActionInstallComponent, "hello", ""),
		planHelper(materia.ActionInstallResource, "hello", "hello.container"),
		planHelper(materia.ActionInstallResource, "hello", "hello.env"),
		planHelper(materia.ActionInstallResource, "hello", "hello.volume"),
		planHelper(materia.ActionInstallResource, "hello", "test.env"),
		planHelper(materia.ActionStartService, "double", "goodbye.service"),
		planHelper(materia.ActionStartService, "hello", "hello.service"),
	}
	assert.Equal(t, len(expectedPlan), len(plan))
	for k, v := range plan {
		expected := expectedPlan[k]
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

func TestExecute(t *testing.T) {
	m := testMateria([]string{"hello.service", "double.service", "goodbye.service"})
	manifest, facts, err := m.Facts(ctx, cfg)
	assert.Nil(t, err)
	assert.NotNil(t, manifest)
	assert.NotNil(t, facts)
	assert.Equal(t, facts, &materia.Facts{
		Hostname: "localhost",
		Roles:    nil,
	})
	expectedManifest := &materia.MateriaManifest{
		Secrets: "age",
		Hosts:   map[string]materia.Host{},
	}
	expectedManifest.Hosts["localhost"] = materia.Host{
		Components: []string{"hello", "double"},
	}
	assert.Equal(t, expectedManifest.Hosts, manifest.Hosts)
	assert.Equal(t, expectedManifest.Secrets, manifest.Secrets)
	fixAgeManifest(manifest)
	err = m.Prepare(ctx, manifest)
	assert.Nil(t, err, fmt.Sprintf("error preparing: %v", err))
	plan, err := m.Plan(ctx, manifest, facts)
	assert.Nil(t, err)
	if err != nil {
		t.Fail()
	}
	expectedPlan := []materia.Action{
		planHelper(materia.ActionInstallComponent, "double", ""),
		planHelper(materia.ActionInstallResource, "double", "MANIFEST.toml"),
		planHelper(materia.ActionInstallResource, "double", "goodbye.container"),
		planHelper(materia.ActionInstallResource, "double", "hello.container"),
		planHelper(materia.ActionInstallComponent, "hello", ""),
		planHelper(materia.ActionInstallResource, "hello", "hello.container"),
		planHelper(materia.ActionInstallResource, "hello", "hello.env"),
		planHelper(materia.ActionInstallResource, "hello", "hello.volume"),
		planHelper(materia.ActionInstallResource, "hello", "test.env"),
		planHelper(materia.ActionStartService, "double", "goodbye.service"),
		planHelper(materia.ActionStartService, "hello", "hello.service"),
	}
	assert.Equal(t, len(expectedPlan), len(plan))
	for k, v := range plan {
		expected := expectedPlan[k]
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
	err = m.Execute(ctx, facts, plan)
	assert.Nil(t, err, fmt.Sprintf("error executing plan: %v", err))
	if err != nil {
		log.Fatal(err)
	}
	// verify all the files are in place
	for _, v := range plan {
		switch v.Todo {
		case materia.ActionInstallComponent:
			_, err := os.Stat(fmt.Sprintf("%v/components/%v", prefix, v.Parent.Name))
			assert.Nil(t, err, fmt.Sprintf("error component not found: %v", v.Payload.Name))
			_, err = os.Stat(fmt.Sprintf("%v/%v", installdir, v.Parent.Name))
			assert.Nil(t, err, fmt.Sprintf("error component not found: %v", v.Payload.Name))

		case materia.ActionInstallResource:
			var dest string
			if v.Payload.Kind == materia.ResourceTypeFile || v.Payload.Kind == materia.ResourceTypeManifest {
				dest = fmt.Sprintf("%v/components/%v/%v", prefix, v.Parent.Name, v.Payload.Name)
			} else {
				dest = fmt.Sprintf("%v/%v/%v", installdir, v.Parent.Name, v.Payload.Name)
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

func planHelper(todo materia.ActionType, name, res string) materia.Action {
	act := materia.Action{
		Todo: todo,
		Parent: &materia.Component{
			Name: name,
		},
		Payload: materia.Resource{
			Name: res,
		},
	}
	return act
}

func fixAgeManifest(m *materia.MateriaManifest) {
	config := m.SecretsConfig.(age.Config)
	config.IdentPath = fmt.Sprintf("%v/source/key.txt", prefix)
	m.SecretsConfig = config
}
