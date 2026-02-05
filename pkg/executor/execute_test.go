package executor

import (
	"bytes"
	"context"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/internal/mocks"
	"primamateria.systems/materia/internal/services"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/plan"
)

func newResSet(resources ...components.Resource) *components.ResourceSet {
	rs := components.NewResourceSet()
	for _, v := range resources {
		_ = rs.Add(v)
	}
	return rs
}

func newServSet(services ...manifests.ServiceResourceConfig) *components.ServiceSet {
	ss := components.NewServiceSet()
	for _, v := range services {
		ss.Add(v)
	}
	return ss
}

func TestExecute(t *testing.T) {
	ctx := context.Background()
	hm := mocks.NewMockHostManager(t)

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
		Resources: newResSet(containerResource, dataResource, manifestResource),
		Services:  newServSet(),
		State:     components.StateFresh,
		Defaults:  map[string]any{},
		Version:   components.DefaultComponentVersion,
	}
	planSteps := []actions.Action{
		{
			Todo:        actions.ActionInstall,
			Parent:      helloComp,
			Target:      components.Resource{Parent: helloComp.Name, Kind: components.ResourceTypeComponent, Path: helloComp.Name},
			DiffContent: getDiffs("", ""),
		},
		{
			Todo:        actions.ActionInstall,
			Parent:      helloComp,
			Target:      containerResource,
			DiffContent: getDiffs("", "[Container]"),
		},
		{
			Todo:        actions.ActionInstall,
			Parent:      helloComp,
			Target:      dataResource,
			DiffContent: getDiffs("", "FOO=BAR"),
		},
		{
			Todo:        actions.ActionInstall,
			Parent:      helloComp,
			Target:      manifestResource,
			DiffContent: getDiffs("", ""),
		},
		{
			Todo:   actions.ActionReload,
			Parent: components.NewComponent("root"),
			Target: components.Resource{Kind: components.ResourceTypeHost},
		},
	}
	plan := plan.NewPlan()
	for _, p := range planSteps {
		assert.NoError(t, plan.Add(p), "can't add action to plan")
	}
	hm.EXPECT().InstallComponent(helloComp).Return(nil)
	hm.EXPECT().InstallResource(containerResource, bytes.NewBufferString("[Container]")).Return(nil)
	hm.EXPECT().InstallResource(dataResource, bytes.NewBufferString("FOO=BAR")).Return(nil)
	hm.EXPECT().InstallResource(manifestResource, bytes.NewBufferString("")).Return(nil)
	hm.EXPECT().Apply(ctx, "", services.ServiceReloadUnits, 0).Return(nil)
	e := &Executor{host: hm}

	steps, err := e.Execute(ctx, plan)
	assert.NoError(t, err)
	assert.Equal(t, plan.Size(), steps, "Missed steps: %v != %v", steps, plan.Size())
}

func getDiffs(res1, res2 string) []diffmatchpatch.Diff {
	dmp := diffmatchpatch.New()
	return dmp.DiffMain(res1, res2, false)
}
