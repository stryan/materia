package executor

import (
	"context"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/mocks"
	"primamateria.systems/materia/pkg/plan"
	"primamateria.systems/materia/pkg/services"
)

func newResSet(resources ...components.Resource) *components.ResourceSet {
	rs := components.NewResourceSet()
	for _, v := range resources {
		_ = rs.Add(v)
	}
	return rs
}

func newServSet(services ...manifests.ServiceResourceConfig) *components.ServiceConfigSet {
	ss := components.NewServiceConfigSet()
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
		Name:           "hello",
		Resources:      newResSet(containerResource, dataResource, manifestResource),
		ServiceConfigs: newServSet(),
		State:          components.StateFresh,
		Defaults:       map[string]any{},
		Version:        components.DefaultComponentVersion,
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
	hm.EXPECT().InstallResource(containerResource, []byte("[Container]")).Return(nil)
	hm.EXPECT().InstallResource(dataResource, []byte("FOO=BAR")).Return(nil)
	hm.EXPECT().InstallResource(manifestResource, []byte("")).Return(nil)
	hm.EXPECT().ApplyService(ctx, "", services.ServiceReloadUnits, 0).Return(nil)
	e := &Executor{host: hm}

	steps, err := e.Execute(ctx, plan)
	assert.NoError(t, err)
	assert.Equal(t, plan.Size(), steps, "Missed steps: %v != %v", steps, plan.Size())
}

func TestExecute_Services(t *testing.T) {
	tests := []struct {
		name     string
		input    actions.Action
		expected func(t *testing.T, hm *mocks.MockHostManager)
	}{
		{
			name: "start - service",
			input: actions.Action{
				Todo:   actions.ActionStart,
				Parent: &components.Component{},
				Target: components.Resource{
					Path:       "hello.service",
					HostObject: "hello.service",
					Parent:     "hello",
					Kind:       components.ResourceTypeService,
				},
			},
			expected: func(t *testing.T, hm *mocks.MockHostManager) {
				hm.EXPECT().ApplyService(mock.Anything, "hello.service", services.ServiceStart, 0).Return(nil)
			},
		},
		{
			name: "stop - service",
			input: actions.Action{
				Todo:   actions.ActionStop,
				Parent: &components.Component{},
				Target: components.Resource{
					Path:       "hello.service",
					HostObject: "hello.service",
					Parent:     "hello",
					Kind:       components.ResourceTypeService,
				},
			},
			expected: func(t *testing.T, hm *mocks.MockHostManager) {
				hm.EXPECT().ApplyService(mock.Anything, "hello.service", services.ServiceStop, 0).Return(nil)
			},
		},
		{
			name: "restart - service",
			input: actions.Action{
				Todo:   actions.ActionRestart,
				Parent: &components.Component{},
				Target: components.Resource{
					Path:       "hello.service",
					HostObject: "hello.service",
					Parent:     "hello",
					Kind:       components.ResourceTypeService,
				},
			},
			expected: func(t *testing.T, hm *mocks.MockHostManager) {
				hm.EXPECT().ApplyService(mock.Anything, "hello.service", services.ServiceRestart, 0).Return(nil)
			},
		},
		{
			name: "reload - service",
			input: actions.Action{
				Todo:   actions.ActionReload,
				Parent: &components.Component{},
				Target: components.Resource{
					Path:       "hello.service",
					HostObject: "hello.service",
					Parent:     "hello",
					Kind:       components.ResourceTypeService,
				},
			},
			expected: func(t *testing.T, hm *mocks.MockHostManager) {
				hm.EXPECT().ApplyService(mock.Anything, "hello.service", services.ServiceReloadService, 0).Return(nil)
			},
		},
		{
			name: "start - container",
			input: actions.Action{
				Todo:   actions.ActionStart,
				Parent: &components.Component{},
				Target: components.Resource{
					Path:       "hello.container",
					HostObject: "systemd-hello",
					Parent:     "hello",
					Kind:       components.ResourceTypeContainer,
				},
			},
			expected: func(t *testing.T, hm *mocks.MockHostManager) {
				hm.EXPECT().ApplyService(mock.Anything, "hello.service", services.ServiceStart, 0).Return(nil)
			},
		},
		{
			name: "stop - container",
			input: actions.Action{
				Todo:   actions.ActionStop,
				Parent: &components.Component{},
				Target: components.Resource{
					Path:       "hello.container",
					HostObject: "systemd-hello",
					Parent:     "hello",
					Kind:       components.ResourceTypeContainer,
				},
			},
			expected: func(t *testing.T, hm *mocks.MockHostManager) {
				hm.EXPECT().ApplyService(mock.Anything, "hello.service", services.ServiceStop, 0).Return(nil)
			},
		},
		{
			name: "restart - container",
			input: actions.Action{
				Todo:   actions.ActionRestart,
				Parent: &components.Component{},
				Target: components.Resource{
					Path:       "hello.container",
					HostObject: "systemd-hello",
					Parent:     "hello",
					Kind:       components.ResourceTypeContainer,
				},
			},
			expected: func(t *testing.T, hm *mocks.MockHostManager) {
				hm.EXPECT().ApplyService(mock.Anything, "hello.service", services.ServiceRestart, 0).Return(nil)
			},
		},
		{
			name: "reload - container",
			input: actions.Action{
				Todo:   actions.ActionReload,
				Parent: &components.Component{},
				Target: components.Resource{
					Path:       "hello.container",
					HostObject: "systemd-hello",
					Parent:     "hello",
					Kind:       components.ResourceTypeContainer,
				},
			},
			expected: func(t *testing.T, hm *mocks.MockHostManager) {
				hm.EXPECT().ApplyService(mock.Anything, "hello.service", services.ServiceReloadService, 0).Return(nil)
			},
		},
		{
			name: "start - pod",
			input: actions.Action{
				Todo:   actions.ActionStart,
				Parent: &components.Component{},
				Target: components.Resource{
					Path:       "hello.pod",
					HostObject: "systemd-hello",
					Parent:     "hello",
					Kind:       components.ResourceTypePod,
				},
			},
			expected: func(t *testing.T, hm *mocks.MockHostManager) {
				hm.EXPECT().ApplyService(mock.Anything, "hello-pod.service", services.ServiceStart, 0).Return(nil)
			},
		},
		{
			name: "stop - pod",
			input: actions.Action{
				Todo:   actions.ActionStop,
				Parent: &components.Component{},
				Target: components.Resource{
					Path:       "hello.pod",
					HostObject: "systemd-hello",
					Parent:     "hello",
					Kind:       components.ResourceTypePod,
				},
			},
			expected: func(t *testing.T, hm *mocks.MockHostManager) {
				hm.EXPECT().ApplyService(mock.Anything, "hello-pod.service", services.ServiceStop, 0).Return(nil)
			},
		},
		{
			name: "restart - pod",
			input: actions.Action{
				Todo:   actions.ActionRestart,
				Parent: &components.Component{},
				Target: components.Resource{
					Path:       "hello.pod",
					HostObject: "systemd-hello",
					Parent:     "hello",
					Kind:       components.ResourceTypePod,
				},
			},
			expected: func(t *testing.T, hm *mocks.MockHostManager) {
				hm.EXPECT().ApplyService(mock.Anything, "hello-pod.service", services.ServiceRestart, 0).Return(nil)
			},
		},
		{
			name: "reload - pod",
			input: actions.Action{
				Todo:   actions.ActionReload,
				Parent: &components.Component{},
				Target: components.Resource{
					Path:       "hello.pod",
					HostObject: "systemd-hello",
					Parent:     "hello",
					Kind:       components.ResourceTypePod,
				},
			},
			expected: func(t *testing.T, hm *mocks.MockHostManager) {
				hm.EXPECT().ApplyService(mock.Anything, "hello-pod.service", services.ServiceReloadService, 0).Return(nil)
			},
		},
	}
	for _, tt := range tests {
		mhm := mocks.NewMockHostManager(t)
		tt.expected(t, mhm)
		e := Executor{host: mhm}
		assert.Nil(t, e.executeAction(context.Background(), tt.input))
	}
}

func getDiffs(res1, res2 string) []diffmatchpatch.Diff {
	dmp := diffmatchpatch.New()
	return dmp.DiffMain(res1, res2, false)
}
