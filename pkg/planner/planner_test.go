package planner

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/mocks"
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

var testComponents = []*components.Component{
	{
		Name:           "hello",
		State:          components.StateFresh,
		Resources:      newResSet(testResources[6]),
		ServiceConfigs: newServSet(),
	},
	{
		Name:      "hello",
		State:     components.StateFresh,
		Resources: newResSet(testResources[0], testResources[6]),
		ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
			Service: "hello.service",
			Static:  false,
		}),
	},
	{
		Name:      "hello",
		State:     components.StateFresh,
		Resources: newResSet(testResources[0], testResources[1], testResources[2], testResources[5]),
		ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
			Service: "hello.service",
			Static:  false,
		}),
	},
	{
		Name:           "oldhello",
		State:          components.StateStale,
		Resources:      newResSet(testResources[0]),
		ServiceConfigs: newServSet(),
	},
	{
		Name:      "updated",
		State:     components.StateMayNeedUpdate,
		Resources: newResSet(testResources[0], testResources[3]),
		ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
			Service: "hello.service",
			Static:  false,
		}),
	},
	{
		Name:           "hello-secret",
		State:          components.StateFresh,
		Resources:      newResSet(testResources[0], testResources[6], testResources[7]),
		ServiceConfigs: newServSet(),
	},
}

var testResources = []components.Resource{
	{
		Path:       "hello.container",
		Parent:     "hello",
		Kind:       components.ResourceTypeContainer,
		Template:   true,
		Content:    "[Container]\nImage=docker.io/materia/hello:latest",
		HostObject: "systemd-hello",
	},
	{
		Path:     "hello.env",
		Parent:   "hello",
		Kind:     components.ResourceTypeFile,
		Template: true,
	},
	{
		Path:     "hello.sh",
		Parent:   "hello",
		Kind:     components.ResourceTypeScript,
		Template: false,
	},
	{
		Path:     manifests.MateriaManifestFile,
		Parent:   "updated",
		Kind:     components.ResourceTypeManifest,
		Template: false,
	},
	{
		Path:       "goodbye.container",
		Parent:     "goodbye",
		Kind:       components.ResourceTypeContainer,
		Template:   true,
		HostObject: "systemd-goodbye",
	},
	{
		Path:     "conf/deep.env",
		Parent:   "hello",
		Kind:     components.ResourceTypeFile,
		Template: true,
	},
	{
		Path:     manifests.MateriaManifestFile,
		Parent:   "hello",
		Kind:     components.ResourceTypeManifest,
		Template: false,
	},
	{
		Path:     "secret",
		Parent:   "hello",
		Kind:     components.ResourceTypePodmanSecret,
		Template: false,
	},
	{
		Path:       "hello.container",
		Parent:     "hello",
		Kind:       components.ResourceTypeContainer,
		Template:   false,
		Content:    "[Container]\nImage=hello.image",
		HostObject: "systemd-hello.container",
	},
}

func Test_BuildComponentGraph(t *testing.T) {
	tests := []struct {
		name           string
		installedComps []string
		assignedComps  []string
		expectedError  bool
		validateGraph  func(*testing.T, *ComponentGraph)
	}{
		{
			name:          "happy-path/empty-components",
			expectedError: false,
			validateGraph: func(t *testing.T, graph *ComponentGraph) {
				assert.Empty(t, graph.List())
			},
		},
		{
			name:           "happy-path/full-tree",
			installedComps: []string{"comp1"},
			assignedComps:  []string{"comp1"},
			expectedError:  false,
			validateGraph: func(t *testing.T, graph *ComponentGraph) {
				assert.Len(t, graph.List(), 1)
				tree, err := graph.Get("comp1")
				require.NoError(t, err)
				assert.NotNil(t, tree.Host, "expected non nill host component")
				assert.NotNil(t, tree.Source, "expected non nill source component")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic := make([]*components.Component, 0, len(tt.installedComps))
			ac := make([]*components.Component, 0, len(tt.assignedComps))
			for _, i := range tt.installedComps {
				ic = append(ic, components.NewComponent(i))
			}
			for _, a := range tt.assignedComps {
				ac = append(ac, components.NewComponent(a))
			}
			graph, err := BuildComponentGraph(context.Background(), ic, ac)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.validateGraph != nil {
				tt.validateGraph(t, graph)
			}
		})
	}
}

func TestGenerateFreshComponentResources(t *testing.T) {
	tests := []struct {
		name          string
		component     *components.Component
		expectedError bool
		expectedPlan  []actions.Action
		validatePlan  func(*testing.T, []actions.Action)
	}{
		{
			name: "sad-path/not-fresh",
			component: &components.Component{
				State: components.StateStale,
			},
			expectedError: true,
		},
		{
			name:          "happy-path/resources",
			component:     testComponents[1],
			expectedError: false,
			expectedPlan: []actions.Action{
				{
					Todo:   actions.ActionInstall,
					Target: components.Resource{Path: "hello"},
				},
				{
					Todo:   actions.ActionInstall,
					Target: components.Resource{Path: "hello.container"},
				},
				{
					Todo:   actions.ActionInstall,
					Target: components.Resource{Path: manifests.ComponentManifestFile},
				},
			},
		},
		{
			name:          "happy-path/secrets",
			component:     testComponents[5],
			expectedError: false,
			expectedPlan: []actions.Action{
				{
					Todo:   actions.ActionInstall,
					Target: components.Resource{Path: "hello-secret"},
				},
				{
					Todo:   actions.ActionInstall,
					Target: components.Resource{Path: "hello.container"},
				},

				{
					Todo:   actions.ActionInstall,
					Target: components.Resource{Path: manifests.ComponentManifestFile},
				},
				{
					Todo:   actions.ActionInstall,
					Target: components.Resource{Path: "secret"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions, err := generateFreshComponentResources(tt.component)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			for k, step := range tt.expectedPlan {
				assert.Equal(t, step.Todo, actions[k].Todo)
				assert.Equal(t, step.Target.Path, actions[k].Target.Path)
			}
			assert.Equal(t, len(tt.expectedPlan), len(actions))
			if tt.validatePlan != nil {
				tt.validatePlan(t, actions)
			}
		})
	}
}

func TestGenerateRemovedComponentResources(t *testing.T) {
	tests := []struct {
		name          string
		component     *components.Component
		expectedError bool
		opts          PlannerConfig
		expectedPlan  []actions.Action
		setup         func(comp *components.Component, mhm *mocks.MockHostManager)
		validatePlan  func(*testing.T, []actions.Action)
	}{
		{
			name: "sad-path/not-needed-removal",
			component: &components.Component{
				State: components.StateFresh,
			},
			expectedError: true,
		},
		{
			name: "happy-path/resources",
			component: &components.Component{
				Name:      "hello",
				State:     components.StateNeedRemoval,
				Resources: newResSet(testResources[0], testResources[6]),
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.service",
					Static:  false,
				}),
			},
			expectedError: false,
			expectedPlan: []actions.Action{
				{
					Todo:   actions.ActionRemove,
					Target: components.Resource{Path: "hello.container"},
				},
				{
					Todo:   actions.ActionRemove,
					Target: components.Resource{Path: manifests.ComponentManifestFile},
				},
				{
					Todo:   actions.ActionRemove,
					Target: components.Resource{Path: "hello", Kind: components.ResourceTypeComponent},
				},
			},
		},
		{
			name: "happy-path/secrets",
			component: &components.Component{
				Name:           "hello-secret",
				State:          components.StateNeedRemoval,
				Resources:      newResSet(testResources[0], testResources[6], testResources[7]),
				ServiceConfigs: newServSet(),
			},
			expectedError: false,
			expectedPlan: []actions.Action{
				{
					Todo:   actions.ActionRemove,
					Target: components.Resource{Path: "hello.container"},
				},
				{
					Todo:   actions.ActionRemove,
					Target: components.Resource{Path: "secret"},
				},
				{
					Todo:   actions.ActionRemove,
					Target: components.Resource{Path: manifests.ComponentManifestFile},
				},
				{
					Todo:   actions.ActionRemove,
					Target: components.Resource{Path: "hello-secret", Kind: components.ResourceTypeComponent},
				},
			},
		},
		{
			name: "happy-path/cleanup",
			component: &components.Component{
				Name:  "hello",
				State: components.StateNeedRemoval,
				Resources: newResSet(testResources[0],
					components.Resource{
						Path:       "hello.volume",
						HostObject: "systemd-hello",
						Kind:       components.ResourceTypeVolume,
						Parent:     "hello",
					}, testResources[6]),
				ServiceConfigs: newServSet(),
			},
			setup: func(comp *components.Component, mhm *mocks.MockHostManager) {
				mhm.EXPECT().GetVolume(mock.Anything, "systemd-hello").Return(&containers.Volume{
					Name: "systemd-hello",
				}, nil)
			},
			expectedError: false,
			opts:          PlannerConfig{CleanupQuadlets: true, CleanupVolumes: true},
			expectedPlan: []actions.Action{
				{
					Todo:   actions.ActionRemove,
					Target: components.Resource{Path: "hello.volume"},
				},
				{
					Todo:   actions.ActionCleanup,
					Target: components.Resource{Path: "hello.volume"},
				},
				{
					Todo:   actions.ActionRemove,
					Target: components.Resource{Path: "hello.container"},
				},
				{
					Todo:   actions.ActionRemove,
					Target: components.Resource{Path: manifests.ComponentManifestFile},
				},
				{
					Todo:   actions.ActionRemove,
					Target: components.Resource{Path: "hello", Kind: components.ResourceTypeComponent},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := mocks.NewMockHostManager(t)
			if tt.setup != nil {
				tt.setup(tt.component, hm)
			}
			actions, err := generateRemovedComponentResources(context.Background(), hm, tt.opts, tt.component)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			for k, step := range tt.expectedPlan {
				assert.Equal(t, step.Todo, actions[k].Todo)
				assert.Equal(t, step.Target.Path, actions[k].Target.Path)
			}
			assert.Equal(t, len(tt.expectedPlan), len(actions))
			if tt.validatePlan != nil {
				tt.validatePlan(t, actions)
			}
		})
	}
}

func TestPlanUpdatedComponent(t *testing.T) {
	tests := []struct {
		name         string
		stale, fresh *components.Component
		opts         PlannerConfig
		setup        func(host *mocks.MockHostManager, stale, fresh *components.Component)
		want         []actions.Action
		wantErr      bool
	}{
		{
			name: "happy-path/one-diffs",
			stale: &components.Component{
				Name:    "hello",
				State:   components.StateMayNeedUpdate,
				Version: components.DefaultComponentVersion,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=hello"),
				),
				ServiceConfigs: newServSet(),
			},
			fresh: &components.Component{
				Name:  "hello",
				State: components.StateFresh,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=goodbye"),
				),
				ServiceConfigs: newServSet(),
			},
			setup: func(host *mocks.MockHostManager, stale *components.Component, fresh *components.Component) {
				host.EXPECT().GetService(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: services.StateActive,
				}, nil)
			},
			want: []actions.Action{
				planHelper(actions.ActionUpdate, "hello", "hello.container"),
				planHelper(actions.ActionReload, "root", ""),
				planHelper(actions.ActionRestart, "hello", "hello.container"),
			},
		},
		{
			name: "happy-path/one-diff-only-resources",
			opts: PlannerConfig{OnlyResources: true},
			stale: &components.Component{
				Name:    "hello",
				State:   components.StateMayNeedUpdate,
				Version: components.DefaultComponentVersion,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=hello"),
				),
				ServiceConfigs: newServSet(),
			},
			fresh: &components.Component{
				Name:  "hello",
				State: components.StateFresh,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=goodbye"),
				),
				ServiceConfigs: newServSet(),
			},
			setup: func(host *mocks.MockHostManager, stale *components.Component, fresh *components.Component) {
			},
			want: []actions.Action{
				planHelper(actions.ActionUpdate, "hello", "hello.container"),
			},
		},
		{
			name: "happy-path/pre-script",
			stale: &components.Component{
				Name:    "hello",
				State:   components.StateMayNeedUpdate,
				Version: components.DefaultComponentVersion,
				Settings: manifests.Settings{
					PreScript: "pre-sync-hook.sh",
				},
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=hello"),
					resourceHelper("pre-sync-hook.sh", "hello", "echo 'hi mom!'"),
				),
				ServiceConfigs: newServSet(),
			},
			fresh: &components.Component{
				Name:  "hello",
				State: components.StateFresh,
				Settings: manifests.Settings{
					PreScript: "pre-sync-hook.sh",
				},
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=goodbye"),
					resourceHelper("pre-sync-hook.sh", "hello", "echo 'hi mom!'"),
				),
				ServiceConfigs: newServSet(),
			},
			setup: func(host *mocks.MockHostManager, stale *components.Component, fresh *components.Component) {
				host.EXPECT().GetService(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: services.StateActive,
				}, nil)
			},
			want: []actions.Action{
				planHelper(actions.ActionExecute, "hello", "pre-sync-hook.sh"),
				planHelper(actions.ActionUpdate, "hello", "hello.container"),
				planHelper(actions.ActionReload, "root", ""),
				planHelper(actions.ActionRestart, "hello", "hello.container"),
			},
		},
		{
			name: "happy-path/post-script",
			stale: &components.Component{
				Name:    "hello",
				State:   components.StateMayNeedUpdate,
				Version: components.DefaultComponentVersion,
				Settings: manifests.Settings{
					PostScript: "post-sync-hook.sh",
				},
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=hello"),
					resourceHelper("post-sync-hook.sh", "hello", "echo 'hi mom!'"),
				),
				ServiceConfigs: newServSet(),
			},
			fresh: &components.Component{
				Name:  "hello",
				State: components.StateFresh,
				Settings: manifests.Settings{
					PostScript: "post-sync-hook.sh",
				},
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=goodbye"),
					resourceHelper("post-sync-hook.sh", "hello", "echo 'hi mom!'"),
				),
				ServiceConfigs: newServSet(),
			},
			setup: func(host *mocks.MockHostManager, stale *components.Component, fresh *components.Component) {
				host.EXPECT().GetService(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: services.StateActive,
				}, nil)
			},
			want: []actions.Action{
				planHelper(actions.ActionUpdate, "hello", "hello.container"),
				planHelper(actions.ActionReload, "root", ""),
				planHelper(actions.ActionExecute, "hello", "post-sync-hook.sh"),
				planHelper(actions.ActionRestart, "hello", "hello.container"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := mocks.NewMockHostManager(t)
			tt.setup(hm, tt.stale, tt.fresh)
			p := NewPlanner(tt.opts, hm)
			got, gotErr := p.PlanUpdatedComponent(context.Background(), &ComponentTree{
				Source: tt.fresh,
				Host:   tt.stale,
			})
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("PlanUpdatedComponent() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("PlanUpdatedComponent() succeeded unexpectedly")
			}

			for k, v := range tt.want {
				if k >= len(got) {
					t.Log(got)
					t.Errorf("Missing step #%v: %v", k, v)
				}
				assert.Equal(t, v.Todo, got[k].Todo)
				assert.Equal(t, v.Target.Path, got[k].Target.Path)
			}
		})
	}
}

func TestGenerateUpdatedComponentResources(t *testing.T) {
	tests := []struct {
		name         string
		stale, fresh *components.Component
		opts         PlannerConfig
		setup        func(host *mocks.MockHostManager, stale, fresh *components.Component)
		want         []actions.Action
		wantErr      bool
	}{
		{
			name: "happy-path/no-diffs",
			stale: &components.Component{
				Name:  "hello",
				State: components.StateMayNeedUpdate,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=hello"),
				),
				ServiceConfigs: newServSet(),
			},
			fresh: &components.Component{
				Name:  "hello",
				State: components.StateFresh,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=hello"),
				),
				ServiceConfigs: newServSet(),
			},
			setup: func(host *mocks.MockHostManager, stale *components.Component, fresh *components.Component) {
			},
			want:    []actions.Action{},
			wantErr: false,
		},
		{
			name: "happy-path/one-diffs",
			stale: &components.Component{
				Name:  "hello",
				State: components.StateMayNeedUpdate,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=hello"),
				),
				ServiceConfigs: newServSet(),
			},
			fresh: &components.Component{
				Name:  "hello",
				State: components.StateFresh,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=goodbye"),
				),
				ServiceConfigs: newServSet(),
			},
			setup: func(host *mocks.MockHostManager, stale *components.Component, fresh *components.Component) {
			},
			want: []actions.Action{
				planHelper(actions.ActionUpdate, "hello", "hello.container"),
			},
			wantErr: false,
		},
		{
			name: "happy-path/removal",
			stale: &components.Component{
				Name:  "hello",
				State: components.StateMayNeedUpdate,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=hello"),
				),
				ServiceConfigs: newServSet(),
			},
			fresh: &components.Component{
				Name:  "hello",
				State: components.StateFresh,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
				),
				ServiceConfigs: newServSet(),
			},
			setup: func(host *mocks.MockHostManager, stale *components.Component, fresh *components.Component) {
			},
			want: []actions.Action{
				planHelper(actions.ActionRemove, "hello", "hello.container"),
			},
			wantErr: false,
		},
		{
			name: "happy-path/add",
			stale: &components.Component{
				Name:  "hello",
				State: components.StateMayNeedUpdate,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
				),
				ServiceConfigs: newServSet(),
			},
			fresh: &components.Component{
				Name:  "hello",
				State: components.StateFresh,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=goodbye"),
				),
				ServiceConfigs: newServSet(),
			},
			setup: func(host *mocks.MockHostManager, stale *components.Component, fresh *components.Component) {
			},
			want: []actions.Action{
				planHelper(actions.ActionInstall, "hello", "hello.container"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := mocks.NewMockHostManager(t)
			tt.setup(hm, tt.stale, tt.fresh)
			got, gotErr := generateUpdatedComponentResources(context.Background(), hm, tt.opts, tt.stale, tt.fresh)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("generateUpdatedComponentResources() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("generateUpdatedComponentResources() succeeded unexpectedly")
			}
			for k, v := range tt.want {
				if k >= len(got) {
					t.Log(got)
					t.Errorf("Missing step #%v: %v", k, v)
				}
				assert.Equal(t, v.Todo, got[k].Todo)
				assert.Equal(t, v.Target.Path, got[k].Target.Path)
			}
		})
	}
}

func Test_GenerateServiceActions(t *testing.T) {
	tests := []struct {
		name         string
		source, host *components.Component
		setup        func(source, host *components.Component, sm *mocks.MockHostManager)
		want         []actions.Action
		wantErr      bool
		validatePlan func(*testing.T, []actions.Action)
	}{
		{
			name:   "fresh - no services",
			source: testComponents[0],
			want:   []actions.Action{},
			setup:  func(source, host *components.Component, services *mocks.MockHostManager) {},
		},
		{
			name:   "fresh - services - none running",
			source: testComponents[1],
			want: []actions.Action{
				{
					Todo: actions.ActionStart,
					Target: components.Resource{
						Path: "hello.service",
					},
				},
			},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				for _, src := range source.ServiceConfigs.List() {
					sm.EXPECT().GetService(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   "inactive",
						Enabled: services.EnableStateDisabled,
					}, nil)
				}
			},
		},
		{
			name:   "fresh - services - running",
			source: testComponents[1],
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				for _, src := range source.ServiceConfigs.List() {
					sm.EXPECT().GetService(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   services.StateActive,
						Enabled: services.EnableStateDisabled,
					}, nil)
				}
			},
		},
		{
			name: "fresh - services - static",
			source: &components.Component{
				Name:      "hello",
				State:     components.StateFresh,
				Resources: newResSet(testResources[0]),
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.service",
					Static:  true,
				}),
			},
			want: []actions.Action{
				{
					Todo: actions.ActionEnable,
					Target: components.Resource{
						Path: "hello.service",
					},
				},
				{
					Todo: actions.ActionStart,
					Target: components.Resource{
						Path: "hello.service",
					},
				},
			},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				for _, src := range source.ServiceConfigs.List() {
					sm.EXPECT().GetService(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   "inactive",
						Enabled: services.EnableStateDisabled,
					}, nil)
				}
			},
		},
		{
			name: "fresh - services - stopped",
			source: &components.Component{
				Name:      "hello",
				State:     components.StateFresh,
				Resources: newResSet(testResources[0]),
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.service",
					Stopped: true,
				}),
			},
			want: []actions.Action{},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				sm.EXPECT().GetService(mock.Anything, "hello.service").Return(&services.Service{
					Name:    "hello.service",
					State:   services.StateUnknown,
					Enabled: services.EnableStateDisabled,
				}, nil)
			},
		},
		{
			name: "fresh - services - container",
			source: &components.Component{
				Name:      "hello",
				State:     components.StateFresh,
				Resources: newResSet(testResources[0]),
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.container",
				}),
			},
			want: []actions.Action{
				{
					Todo: actions.ActionStart,
					Target: components.Resource{
						Path: "hello.container",
					},
				},
			},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				sm.EXPECT().GetService(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: "inactive",
				}, nil)
			},
		},
		{
			name: "fresh - services - container with image",
			source: &components.Component{
				Name:      "hello",
				State:     components.StateFresh,
				Resources: newResSet(testResources[8]),
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.container",
				}),
			},
			want: []actions.Action{
				{
					Todo: actions.ActionStart,
					Target: components.Resource{
						Path: "hello.container",
					},
				},
			},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				sm.EXPECT().GetService(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: "inactive",
				}, nil)
			},
			validatePlan: func(t *testing.T, a []actions.Action) {
				assert.Equal(t, 1, len(a))
				assert.NotNil(t, a[0].Metadata)
				assert.Equal(t, *a[0].Metadata.ServiceTimeout, 60)
			},
		},
		{
			name: "fresh - services - container with defined image",
			source: &components.Component{
				Name:      "hello",
				State:     components.StateFresh,
				Resources: newResSet(testResources[8]),
				ServiceConfigs: newServSet(
					manifests.ServiceResourceConfig{
						Service: "hello.container",
					},
					manifests.ServiceResourceConfig{
						Service: "hello.image",
						Stopped: true,
						Timeout: 100,
					},
				),
			},
			want: []actions.Action{
				{
					Todo: actions.ActionStart,
					Target: components.Resource{
						Path: "hello.container",
					},
				},
			},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				sm.EXPECT().GetService(mock.Anything, "hello-image.service").Return(&services.Service{
					Name:  "hello-image.service",
					State: services.StateActive,
				}, nil)
				sm.EXPECT().GetService(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: services.StateInactive,
				}, nil)
			},
			validatePlan: func(t *testing.T, a []actions.Action) {
				assert.Equal(t, 1, len(a), "expected %v steps got %v", 1, len(a))
				assert.NotNil(t, a[0].Metadata)
				assert.Equal(t, *a[0].Metadata.ServiceTimeout, 160)
			},
		},

		{
			name:  "removal - no services",
			host:  testComponents[0],
			want:  []actions.Action{},
			setup: func(source, host *components.Component, services *mocks.MockHostManager) {},
		},
		{
			name: "removal - services - none running",
			host: testComponents[1],
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				for _, src := range host.ServiceConfigs.List() {
					sm.EXPECT().GetService(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   "inactive",
						Enabled: services.EnableStateDisabled,
					}, nil)
				}
			},
		},
		{
			name: "removal - services - running",
			host: testComponents[1],
			want: []actions.Action{
				{
					Todo: actions.ActionStop,
					Target: components.Resource{
						Path: "hello.service",
					},
				},
			},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				for _, src := range host.ServiceConfigs.List() {
					sm.EXPECT().GetService(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   services.StateActive,
						Enabled: services.EnableStateDisabled,
					}, nil)
				}
			},
		},
		{
			name: "removal - services - static",
			host: &components.Component{
				Name:      "hello",
				State:     components.StateNeedRemoval,
				Resources: newResSet(testResources[0]),
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.service",
					Static:  true,
				}),
			},
			want: []actions.Action{
				{
					Todo: actions.ActionStop,
					Target: components.Resource{
						Path: "hello.service",
					},
				},
			},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				for _, src := range host.ServiceConfigs.List() {
					sm.EXPECT().GetService(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   services.StateActive,
						Enabled: services.EnableStateDisabled,
					}, nil)
				}
			},
		},
		{
			name: "removal - services - stopped",
			host: &components.Component{
				Name:      "hello",
				State:     components.StateNeedRemoval,
				Resources: newResSet(testResources[0]),
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.service",
					Stopped: true,
				}),
			},
			want: []actions.Action{},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				for _, src := range host.ServiceConfigs.List() {
					sm.EXPECT().GetService(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   "inactive",
						Enabled: services.EnableStateDisabled,
					}, nil)
				}
			},
		},
		{
			name: "removal - services - container",
			host: &components.Component{
				Name:      "hello",
				State:     components.StateNeedRemoval,
				Resources: newResSet(testResources[0]),
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.container",
				}),
			},
			want: []actions.Action{
				{
					Todo: actions.ActionStop,
					Target: components.Resource{
						Path: "hello.container",
					},
				},
			},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				sm.EXPECT().GetService(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: services.StateActive,
				}, nil)
			},
		},
		{
			name: "removal - services - container with image",
			host: &components.Component{
				Name:      "hello",
				State:     components.StateNeedRemoval,
				Resources: newResSet(testResources[8]),
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.container",
				}),
			},
			want: []actions.Action{
				{
					Todo: actions.ActionStop,
					Target: components.Resource{
						Path: "hello.container",
					},
				},
			},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				sm.EXPECT().GetService(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: services.StateActive,
				}, nil)
			},
			validatePlan: func(t *testing.T, a []actions.Action) {
				assert.Equal(t, 1, len(a))
				assert.NotNil(t, a[0].Metadata)
				assert.Equal(t, *a[0].Metadata.ServiceTimeout, 60)
			},
		},
		{
			name: "removal - services - container with defined image",
			host: &components.Component{
				Name:      "hello",
				State:     components.StateNeedRemoval,
				Resources: newResSet(testResources[8]),
				ServiceConfigs: newServSet(
					manifests.ServiceResourceConfig{
						Service: "hello.container",
					},
					manifests.ServiceResourceConfig{
						Service: "hello.image",
						Stopped: true,
						Timeout: 100,
					},
				),
			},
			want: []actions.Action{
				{
					Todo: actions.ActionStop,
					Target: components.Resource{
						Path: "hello.container",
					},
				},
			},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				sm.EXPECT().GetService(mock.Anything, "hello-image.service").Return(&services.Service{
					Name:  "hello-image.service",
					State: "inactive",
				}, nil)
				sm.EXPECT().GetService(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: services.StateActive,
				}, nil)
			},
			validatePlan: func(t *testing.T, a []actions.Action) {
				assert.Equal(t, 1, len(a))
				assert.NotNil(t, a[0].Metadata)
				assert.Equal(t, *a[0].Metadata.ServiceTimeout, 160)
			},
		},
		{
			name: "updated - new service",
			host: &components.Component{
				Name:  "foo",
				State: components.StateNeedUpdate,
				Resources: newResSet(components.Resource{
					Path:   "test.service",
					Parent: "foo",
					Kind:   components.ResourceTypeService,
				}),
				ServiceConfigs: newServSet(),
			},
			source: &components.Component{
				Name:  "foo",
				State: components.StateNeedUpdate,
				Resources: newResSet(components.Resource{
					Path:   "test.service",
					Parent: "foo",
					Kind:   components.ResourceTypeService,
				}),
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service: "test.service",
				}),
			},
			want: []actions.Action{
				{
					Todo: actions.ActionStart,
					Target: components.Resource{
						Path:   "test.service",
						Parent: "foo",
						Kind:   components.ResourceTypeService,
					},
				},
			},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				sm.EXPECT().GetService(mock.Anything, "test.service").Return(&services.Service{
					Name:  "test.service",
					State: services.StateInactive,
				}, nil)
			},
		},
		{
			name: "updated - removed service",
			host: &components.Component{
				Name:  "foo",
				State: components.StateNeedUpdate,
				Resources: newResSet(components.Resource{
					Path:   "test.service",
					Parent: "foo",
					Kind:   components.ResourceTypeService,
				}),
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service: "test.service",
				}),
			},
			source: &components.Component{
				Name:  "foo",
				State: components.StateNeedUpdate,
				Resources: newResSet(components.Resource{
					Path:   "test.service",
					Parent: "foo",
					Kind:   components.ResourceTypeService,
				}),

				ServiceConfigs: newServSet(),
			},
			want: []actions.Action{
				{
					Todo: actions.ActionStop,
					Target: components.Resource{
						Path:   "test.service",
						Parent: "foo",
						Kind:   components.ResourceTypeService,
					},
				},
			},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				sm.EXPECT().GetService(mock.Anything, "test.service").Return(&services.Service{
					Name:  "test.service",
					State: services.StateActive,
				}, nil)
			},
		},
		{
			name: "updated - start stopped",
			host: &components.Component{
				Name:  "foo",
				State: components.StateNeedUpdate,
				Resources: newResSet(components.Resource{
					Path:   "test.service",
					Parent: "foo",
					Kind:   components.ResourceTypeService,
				}),
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service: "test.service",
				}),
			},
			source: &components.Component{
				Name:  "foo",
				State: components.StateNeedUpdate,
				Resources: newResSet(components.Resource{
					Path:   "test.service",
					Parent: "foo",
					Kind:   components.ResourceTypeService,
				}),
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service: "test.service",
				}),
			},
			want: []actions.Action{
				{
					Todo: actions.ActionStart,
					Target: components.Resource{
						Path:   "test.service",
						Parent: "foo",
						Kind:   components.ResourceTypeService,
					},
				},
			},
			setup: func(source, host *components.Component, sm *mocks.MockHostManager) {
				sm.EXPECT().GetService(mock.Anything, "test.service").Return(&services.Service{
					Name:  "test.service",
					State: services.StateInactive,
				}, nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := mocks.NewMockHostManager(t)
			tt.setup(tt.source, tt.host, hm)
			got, gotErr := GenerateServiceActions(context.Background(), hm, tt.source, tt.host)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GenerateServiceActions failed: %v", gotErr)
				}
				return
			}

			if tt.wantErr {
				t.Fatal("GenerateServiceActions succeeded unexpectedly")
			}
			for k, v := range tt.want {
				if k >= len(got) {
					t.Logf("Steps got: %v", got)
					t.Errorf("Missing step #%v: %v", k, v)
				}
				assert.Equal(t, v.Todo, got[k].Todo, "Todo %v != expected Todo %v", got[k].Todo, v.Todo)
				assert.Equal(t, v.Target.Path, got[k].Target.Path)
			}
			if tt.validatePlan != nil {
				tt.validatePlan(t, got)
			}
		})
	}
}

func Test_generateComponentServiceTriggers(t *testing.T) {
	tests := []struct {
		name    string
		input   *components.Component
		want    map[string][]actions.Action
		wantErr bool
	}{
		{
			name: "base case",
			input: &components.Component{
				ServiceConfigs: newServSet(),
				Resources:      newResSet(),
			},
			want: make(map[string][]actions.Action),
		},
		{
			name: "restarted",
			input: &components.Component{
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service:     "hello.service",
					RestartedBy: []string{"hello.env"},
				}),
				Resources: newResSet(),
			},
			want: map[string][]actions.Action{
				"hello.env": {
					{
						Todo: actions.ActionRestart,
						Target: components.Resource{
							Path: "hello.service",
						},
					},
				},
			},
		},
		{
			name: "reloaded",
			input: &components.Component{
				ServiceConfigs: newServSet(manifests.ServiceResourceConfig{
					Service:    "hello.service",
					ReloadedBy: []string{"hello.env"},
				}),
				Resources: newResSet(),
			},
			want: map[string][]actions.Action{
				"hello.env": {
					{
						Todo: actions.ActionReload,
						Target: components.Resource{
							Path: "hello.service",
						},
					},
				},
			},
		},
		{
			name: "auto-restart containers",
			input: &components.Component{
				ServiceConfigs: newServSet(),
				Resources:      newResSet(resourceHelper("hello.container", "hello", "[Container]\nImage=foo")),
			},
			want: map[string][]actions.Action{
				"hello.container": {
					{
						Todo: actions.ActionRestart,
						Target: components.Resource{
							Path: "hello.container",
						},
					},
				},
			},
		},
		{
			name: "don't auto restart when set",
			input: &components.Component{
				ServiceConfigs: newServSet(),
				Resources:      newResSet(resourceHelper("hello.container", "hello", "[Container]\nImage=foo")),
				Settings: manifests.Settings{
					NoRestart: true,
				},
			},
			want: map[string][]actions.Action{},
		},

		{
			name: "auto-restart pods",
			input: &components.Component{
				ServiceConfigs: newServSet(),
				Resources:      newResSet(resourceHelper("hello.pod", "hello", "[Pod]\n")),
			},
			want: map[string][]actions.Action{
				"hello.pod": {
					{
						Todo: actions.ActionRestart,
						Target: components.Resource{
							Path: "hello.pod",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := generateComponentServiceTriggers(tt.input)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GenerateServiceActions failed: %v", gotErr)
				}
				return
			}

			if tt.wantErr {
				t.Fatal("GenerateServiceActions succeeded unexpectedly")
			}
			for k, v := range got {
				expected, ok := tt.want[k]
				if !ok {
					t.Errorf("Missing resource trigger for %v", k)
				}
				for i, gotv := range v {
					exepctedv := expected[i]
					assert.Equal(t, exepctedv.Todo, gotv.Todo, "Expected %v got %v", exepctedv.Todo, gotv.Todo)
					assert.Equal(t, exepctedv.Target.Path, gotv.Target.Path, "Expected path %v got %v", exepctedv.Target.Path, gotv.Target.Path)
				}
			}
		})
	}
}

func Test_processTriggeredUpdates(t *testing.T) {
	tests := []struct {
		name         string
		input        []actions.Action
		triggers     map[string][]actions.Action
		comp         *components.Component
		setup        func(sm *mocks.MockHostManager)
		want         []actions.Action
		wantErr      bool
		validatePlan func(*testing.T, []actions.Action)
	}{
		{
			name: "base case",
			input: []actions.Action{
				planHelper(actions.ActionUpdate, "hello", "hello.env"),
			},
			triggers: make(map[string][]actions.Action),
			comp: &components.Component{
				Name: "hello",
			},
			setup: func(sm *mocks.MockHostManager) {},
		},
		{
			name: "action from trigger",
			input: []actions.Action{
				planHelper(actions.ActionUpdate, "hello", "hello.env"),
			},
			triggers: map[string][]actions.Action{
				"hello.env": {
					{
						Todo: actions.ActionRestart,
						Target: components.Resource{
							Path: "hello.service",
							Kind: components.ResourceTypeService,
						},
					},
				},
			},
			comp: &components.Component{
				Name: "hello",
				ServiceConfigs: newServSet(
					manifests.ServiceResourceConfig{
						Service:     "hello.service",
						RestartedBy: []string{"hello.env"},
					},
				),
			},
			setup: func(sm *mocks.MockHostManager) {
				sm.EXPECT().GetService(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: services.StateActive,
				}, nil)
			},
			want: []actions.Action{
				{
					Todo: actions.ActionRestart,
					Target: components.Resource{
						Path: "hello.service",
					},
				},
			},
		},
		{
			name: "action from component setting",
			input: []actions.Action{
				planHelper(actions.ActionUpdate, "hello", "hello.container"),
			},
			triggers: map[string][]actions.Action{
				"hello.container": {
					{
						Todo: actions.ActionRestart,
						Target: components.Resource{
							Path: "hello.container",
							Kind: components.ResourceTypeContainer,
						},
					},
				},
			},
			comp: &components.Component{
				Name: "hello",
				ServiceConfigs: newServSet(
					manifests.ServiceResourceConfig{
						Service: "hello.container",
					},
				),
			},
			setup: func(sm *mocks.MockHostManager) {
				sm.EXPECT().GetService(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: services.StateActive,
				}, nil)
			},
			want: []actions.Action{
				{
					Todo: actions.ActionRestart,
					Target: components.Resource{
						Path: "hello.container",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := mocks.NewMockHostManager(t)
			tt.setup(hm)
			got, gotErr := processTriggeredUpdates(context.Background(), hm, tt.comp, tt.triggers, tt.input)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("processTriggeredUpdates failed: %v", gotErr)
				}
				return
			}

			if tt.wantErr {
				t.Fatal("processTriggeredUpdates succeeded unexpectedly")
			}
			for k, v := range tt.want {
				if k >= len(got) {
					t.Logf("Steps got: %v", got)
					t.Errorf("Missing step #%v: %v", k, v)
				}
				assert.Equal(t, v.Todo, got[k].Todo, "Todo %v != expected Todo %v", got[k].Todo, v.Todo)
				assert.Equal(t, v.Target.Path, got[k].Target.Path)
			}
			if tt.validatePlan != nil {
				tt.validatePlan(t, got)
			}
		})
	}
}

func resourceHelper(name, parent, content string) components.Resource {
	var result components.Resource
	result.Path = strings.TrimSuffix(name, ".gotmpl")
	result.Template = components.IsTemplate(name)
	result.Parent = parent
	result.Kind = components.FindResourceType(result.Path)
	result.Content = content
	if result.Kind != components.ResourceTypeImage && result.Kind != components.ResourceTypeBuild {
		result.HostObject = fmt.Sprintf("systemd-%v", strings.TrimSuffix(filepath.Base(result.Path), filepath.Ext(result.Path)))
	}
	return result
}

func TestPlan(t *testing.T) {
	expected := []actions.Action{
		planHelper(actions.ActionInstall, "hello", ""),
		planHelper(actions.ActionInstall, "hello", "hello.container"),
		planHelper(actions.ActionInstall, "hello", "hello.env"),
		planHelper(actions.ActionInstall, "hello", manifests.ComponentManifestFile),
		planHelper(actions.ActionReload, "", ""),
	}

	ctx := context.Background()
	p := &Planner{}
	containerResource := components.Resource{
		Parent:     "hello",
		Path:       "hello.container",
		Kind:       components.ResourceTypeContainer,
		HostObject: "systemd-hello",
	}
	dataResource := components.Resource{
		Parent: "hello",
		Path:   "hello.env",
		Kind:   components.ResourceTypeFile,
	}
	manifestResource := components.Resource{
		Parent: "hello",
		Path:   manifests.MateriaManifestFile,
		Kind:   components.ResourceTypeManifest,
	}
	helloComp := &components.Component{
		Name:           "hello",
		Resources:      newResSet(containerResource, dataResource, manifestResource),
		State:          components.StateFresh,
		Defaults:       map[string]any{},
		ServiceConfigs: newServSet(),
		Version:        components.DefaultComponentVersion,
	}
	plan, err := p.Plan(ctx, "localhost", []*components.Component{}, []*components.Component{helloComp})
	assert.NoError(t, err)
	for k, v := range plan.Steps() {
		expected := expected[k]
		assert.Equal(t, expected.Todo, v.Todo, "%v Todo not equal: %v != %v", v, v.Todo, expected.Todo)
		assert.Equal(t, expected.Parent.Name, v.Parent.Name, "Res %v Parent not equal: %v != %v", v.Target.Path, v.Parent.Name, expected.Parent.Name)
		assert.Equal(t, expected.Target.Path, v.Target.Path, "Res %v Path not equal: %v != %v", v.Target.Path, v.Target.Path, expected.Target.Path)
	}
}

func planHelper(todo actions.ActionType, name, res string) actions.Action {
	if res == "" {
		if name == "" {
			name = "root"
		}
		if todo == actions.ActionReload {
			return actions.Action{
				Todo:   todo,
				Parent: &components.Component{Name: name},
				Target: components.Resource{
					Parent: name,
					Kind:   components.ResourceTypeHost,
				},
			}
		}
		return actions.Action{
			Todo:   todo,
			Parent: &components.Component{Name: name},
			Target: components.Resource{
				Parent: name,
				Kind:   components.ResourceTypeComponent,
				Path:   name,
			},
		}
	}
	act := actions.Action{
		Todo: todo,
		Parent: &components.Component{
			Name: name,
		},
		Target: components.Resource{
			Parent: name,
			Kind:   components.FindResourceType(res),
			Path:   res,
		},
	}
	return act
}
