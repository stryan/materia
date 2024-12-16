package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git.saintnet.tech/stryan/materia/internal/materia"
	"github.com/stretchr/testify/assert"
)

var (
	ctx                context.Context
	cfg                *materia.Config
	prefix, installdir string
)

func testMateria(services []string) *materia.Materia {
	m, err := materia.NewMateria(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	var mockservices MockServices
	mockservices.Services = make(map[string]string)
	m.Containers = &MockContainers{make(map[string]string)}
	for _, v := range services {
		mockservices.Services[v] = "unknown"
	}
	m.Services = &mockservices
	return m
}

func TestMain(m *testing.M) {
	testPrefix := fmt.Sprintf("/tmp/materia-test-%v", time.Now().Unix())
	prefix = filepath.Join(testPrefix, "materia")
	installdir = filepath.Join(testPrefix, "install")
	cfg = &materia.Config{
		SourceURL:   "file://../example_repo",
		Debug:       false,
		Hostname:    "localhost",
		Timeout:     0,
		Prefix:      testPrefix,
		Destination: installdir,
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
		Role:     "",
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
		Role:     "",
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
	err = m.Prepare(ctx, manifest)
	assert.Nil(t, err, fmt.Sprintf("error preparing: %v", err))
	plan, err := m.Plan(ctx, manifest, facts)
	assert.Nil(t, err)
	if err != nil {
		t.Fail()
	}
	// {Todo:InstallComponent Parent:{c double Fresh } Payload:{Path: Name: Kind:Unknown Template:false}}
	// {Todo:InstallResource Parent:{c double Fresh } Payload:{Path:/tmp/materia-test-1731883729/source/components/double/goodbye.container.gotmpl Name:goodbye.container Kind:Container Template:true}}
	// {Todo:InstallResource Parent:{c double Fresh } Payload:{Path:/tmp/materia-test-1731883729/source/components/double/hello.container.gotmpl Name:hello.container Kind:Container Template:true}}
	// {Todo:InstallComponent Parent:{c hello Fresh } Payload:{Path: Name: Kind:Unknown Template:false}}
	// {Todo:InstallResource Parent:{c hello Fresh } Payload:{Path:/tmp/materia-test-1731883729/source/components/hello/hello.container.gotmpl Name:hello.container Kind:Container Template:true}}
	// {Todo:InstallResource Parent:{c hello Fresh } Payload:{Path:/tmp/materia-test-1731883729/source/components/hello/hello.env Name:hello.env Kind:File Template:false}}
	// {Todo:InstallResource Parent:{c hello Fresh } Payload:{Path:/tmp/materia-test-1731883729/source/components/hello/hello.volume Name:hello.volume Kind:Volume Template:false}}
	// {Todo:InstallResource Parent:{c hello Fresh } Payload:{Path:/tmp/materia-test-1731883729/source/components/hello/test.env.gotmpl Name:test.env Kind:File Template:true}}
	// {Todo:StartService Parent:{c hello Fresh } Payload:{Path: Name:hello-volume.service Kind:Service Template:false}}
	// {Todo:StartService Parent:{c double Fresh } Payload:{Path: Name:goodbye.service Kind:Service Template:false}}
	// {Todo:StartService Parent:{c hello Fresh } Payload:{Path: Name:hello.service Kind:Service Template:false}}
	expectedPlan := []materia.Action{
		planHelper(materia.ActionInstallComponent, "double", ""),
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
		Role:     "",
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
	err = m.Prepare(ctx, manifest)
	assert.Nil(t, err, fmt.Sprintf("error preparing: %v", err))
	plan, err := m.Plan(ctx, manifest, facts)
	assert.Nil(t, err)
	if err != nil {
		t.Fail()
	}
	expectedPlan := []materia.Action{
		planHelper(materia.ActionInstallComponent, "double", ""),
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
	err = m.Execute(ctx, plan)
	assert.Nil(t, err, fmt.Sprintf("error executing plan: %v", err))
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
			if v.Payload.Kind == materia.ResourceTypeFile {
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
