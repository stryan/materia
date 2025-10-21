package materia

import (
	"bytes"
	"context"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
	"primamateria.systems/materia/internal/attributes"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/services"
	"primamateria.systems/materia/pkg/manifests"
)

func TestExecute(t *testing.T) {
	ctx := context.Background()
	sm := NewMockSourceManager(t)
	hm := NewMockHostManager(t)
	v := NewMockAttributesEngine(t)
	man := &manifests.MateriaManifest{
		Hosts: map[string]manifests.Host{
			"localhost": {
				Components: []string{"hello"},
			},
		},
	}
	containerResource := components.Resource{
		Path:   "hello.container",
		Parent: "hello",
		Kind:   components.ResourceTypeContainer,
	}
	dataResource := components.Resource{
		Path:   "hello.env",
		Parent: "hello",
		Kind:   components.ResourceTypeFile,
	}
	manifestResource := components.Resource{
		Path:   manifests.MateriaManifestFile,
		Parent: "hello",
		Kind:   components.ResourceTypeManifest,
	}
	helloComp := &components.Component{
		Name:      "hello",
		Resources: []components.Resource{containerResource, dataResource, manifestResource},
		State:     components.StateFresh,
		Defaults:  map[string]any{},
		Version:   components.DefaultComponentVersion,
	}
	planSteps := []Action{
		{
			Todo:    ActionInstall,
			Parent:  helloComp,
			Target:  components.Resource{Parent: helloComp.Name, Kind: components.ResourceTypeComponent, Path: helloComp.Name},
			Content: getDiffs("", ""),
		},
		{
			Todo:    ActionInstall,
			Parent:  helloComp,
			Target:  containerResource,
			Content: getDiffs("", "[Container]"),
		},
		{
			Todo:    ActionInstall,
			Parent:  helloComp,
			Target:  dataResource,
			Content: getDiffs("", "FOO=BAR"),
		},
		{
			Todo:    ActionInstall,
			Parent:  helloComp,
			Target:  manifestResource,
			Content: getDiffs("", ""),
		},
		{
			Todo:   ActionReload,
			Parent: rootComponent,
			Target: components.Resource{Kind: components.ResourceTypeHost},
		},
	}
	plan := NewPlan([]string{}, []string{})
	for _, p := range planSteps {
		plan.Add(p)
	}
	hm.EXPECT().GetHostname().Return("localhost")
	v.EXPECT().Lookup(ctx, attributes.AttributesFilter{
		Hostname:  "localhost",
		Roles:     []string(nil),
		Component: "root",
	}).Return(map[string]any{})
	v.EXPECT().Lookup(ctx, attributes.AttributesFilter{
		Hostname:  "localhost",
		Roles:     []string(nil),
		Component: "hello",
	}).Return(map[string]any{})

	hm.EXPECT().InstallComponent(helloComp).Return(nil)
	hm.EXPECT().InstallResource(containerResource, bytes.NewBufferString("[Container]")).Return(nil)
	hm.EXPECT().InstallResource(dataResource, bytes.NewBufferString("FOO=BAR")).Return(nil)
	hm.EXPECT().InstallResource(manifestResource, bytes.NewBufferString("")).Return(nil)
	hm.EXPECT().Apply(ctx, "", services.ServiceReloadUnits).Return(nil)
	m := &Materia{Manifest: man, Source: sm, Host: hm, Vault: v, macros: testMacroMap}

	steps, err := m.Execute(ctx, plan)
	assert.NoError(t, err)
	assert.Equal(t, plan.Size(), steps, "Missed steps: %v != %v", steps, plan.Size())
}

func getDiffs(res1, res2 string) []diffmatchpatch.Diff {
	dmp := diffmatchpatch.New()
	return dmp.DiffMain(res1, res2, false)
}
