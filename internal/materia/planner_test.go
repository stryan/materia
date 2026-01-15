package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/internal/attributes"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/services"
	"primamateria.systems/materia/pkg/manifests"
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

var testComponents = []*components.Component{
	{
		Name:      "hello",
		State:     components.StateFresh,
		Resources: newResSet(testResources[6]),
		Services:  newServSet(),
	},
	{
		Name:      "hello",
		State:     components.StateFresh,
		Resources: newResSet(testResources[0], testResources[6]),
		Services: newServSet(manifests.ServiceResourceConfig{
			Service: "hello.service",
			Static:  false,
		}),
	},
	{
		Name:      "hello",
		State:     components.StateFresh,
		Resources: newResSet(testResources[0], testResources[1], testResources[2], testResources[5]),
		Services: newServSet(manifests.ServiceResourceConfig{
			Service: "hello.service",
			Static:  false,
		}),
	},
	{
		Name:      "oldhello",
		State:     components.StateStale,
		Resources: newResSet(testResources[0]),
		Services:  newServSet(),
	},
	{
		Name:      "updated",
		State:     components.StateMayNeedUpdate,
		Resources: newResSet(testResources[0], testResources[3]),
		Services: newServSet(manifests.ServiceResourceConfig{
			Service: "hello.service",
			Static:  false,
		}),
	},
	{
		Name:      "hello-secret",
		State:     components.StateFresh,
		Resources: newResSet(testResources[0], testResources[6], testResources[7]),
		Services:  newServSet(),
	},
}

var testResources = []components.Resource{
	{
		Path:     "hello.container",
		Parent:   "hello",
		Kind:     components.ResourceTypeContainer,
		Template: true,
		Content:  "[Container]\nImage=docker.io/materia/hello:latest",
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
		Path:     "goodbye.container",
		Parent:   "goodbye",
		Kind:     components.ResourceTypeContainer,
		Template: true,
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
		Path:     "hello.container",
		Parent:   "hello",
		Kind:     components.ResourceTypeContainer,
		Template: false,
		Content:  "[Container]\nImage=hello.image",
	},
}

var testSnippets = func() map[string]*Snippet {
	snips := make(map[string]*Snippet)
	defaultSnippets := loadDefaultSnippets()
	for _, v := range defaultSnippets {
		snips[v.Name] = v
	}
	return snips
}

var testMacroMap = func(vars map[string]any) template.FuncMap {
	return template.FuncMap{
		"m_dataDir": func(arg string) (string, error) {
			return filepath.Join(filepath.Join("/var/lib/", "materia", "components"), arg), nil
		},
		"m_facts": func(arg string) (any, error) {
			return "fact!", nil
		},
		"m_default": func(arg string, def string) string {
			val, ok := vars[arg]
			if ok {
				return val.(string)
			}
			return def
		},
		"exists": func(arg string) bool {
			_, ok := vars[arg]
			return ok
		},
		"snippet": func(name string, args ...string) (string, error) {
			s, ok := testSnippets()[name]
			if !ok {
				return "", errors.New("snippet not found")
			}
			snipVars := make(map[string]string, len(s.Parameters))
			for k, v := range s.Parameters {
				snipVars[v] = args[k]
			}

			result := bytes.NewBuffer([]byte{})
			err := s.Body.Execute(result, snipVars)
			return result.String(), err
		},
	}
}

func TestMateria_BuildComponentGraph(t *testing.T) {
	tests := []struct {
		name           string
		installedComps []string
		assignedComps  []string
		setup          func(*MockHostManager, *MockSourceManager, *MockAttributesEngine)
		expectedError  bool
		validateGraph  func(*testing.T, *ComponentGraph)
	}{
		{
			name:          "happy-path/empty-components",
			setup:         func(_ *MockHostManager, _ *MockSourceManager, _ *MockAttributesEngine) {},
			expectedError: false,
			validateGraph: func(t *testing.T, graph *ComponentGraph) {
				assert.Empty(t, graph.List())
			},
		},
		{
			name:           "sad-path/host-component-error",
			installedComps: []string{"comp1"},
			setup: func(mhm *MockHostManager, _ *MockSourceManager, vault *MockAttributesEngine) {
				mhm.EXPECT().GetComponent("comp1").Return(nil, errors.New("bwah?"))
			},
			expectedError: true,
		},
		{
			name:           "happy-path/load-host-component",
			installedComps: []string{"comp1"},
			setup: func(mhm *MockHostManager, _ *MockSourceManager, vault *MockAttributesEngine) {
				comp := &components.Component{
					Name:      "comp1",
					Version:   components.DefaultComponentVersion,
					Resources: newResSet(),
					Services:  newServSet(),
				}
				mhm.EXPECT().GetComponent("comp1").Return(comp, nil)
				manifest := &manifests.ComponentManifest{}
				mhm.EXPECT().GetManifest(comp).Return(manifest, nil)
			},
			expectedError: false,
			validateGraph: func(t *testing.T, graph *ComponentGraph) {
				assert.Len(t, graph.List(), 1)
				tree, err := graph.Get("comp1")
				require.NoError(t, err)
				assert.NotNil(t, tree.host)
				assert.Nil(t, tree.source)
			},
		},
		{
			name:          "sad-path/source-component-error",
			assignedComps: []string{"comp2"},
			setup: func(mhm *MockHostManager, msm *MockSourceManager, vault *MockAttributesEngine) {
				vault.EXPECT().Lookup(mock.Anything, attributes.AttributesFilter{
					Hostname:  "localhost",
					Component: "comp2",
				}).Return(map[string]any{})
				msm.EXPECT().GetComponent("comp2").Return(nil, errors.New("bwah?"))
			},
			expectedError: true,
		},
		{
			name:          "happy-path/load-source-component",
			assignedComps: []string{"comp2"},
			setup: func(_ *MockHostManager, msm *MockSourceManager, vault *MockAttributesEngine) {
				comp := &components.Component{
					Name:      "comp2",
					Resources: newResSet(),
					Services:  newServSet(),
				}
				vault.EXPECT().Lookup(mock.Anything, attributes.AttributesFilter{
					Hostname:  "localhost",
					Component: "comp2",
				}).Return(map[string]any{})
				msm.EXPECT().GetComponent("comp2").Return(comp, nil)
				manifest := &manifests.ComponentManifest{}
				msm.EXPECT().GetManifest(comp).Return(manifest, nil)
			},
			expectedError: false,
			validateGraph: func(t *testing.T, graph *ComponentGraph) {
				assert.Len(t, graph.List(), 1)
				tree, err := graph.Get("comp2")
				require.NoError(t, err)
				assert.Nil(t, tree.host)
				assert.NotNil(t, tree.source)
			},
		},
		{
			name:           "happy-path/full-tree",
			installedComps: []string{"comp1"},
			assignedComps:  []string{"comp1"},
			setup: func(mhm *MockHostManager, msm *MockSourceManager, vault *MockAttributesEngine) {
				hostComp := &components.Component{
					Name:      "comp1",
					Version:   components.DefaultComponentVersion,
					Resources: newResSet(),
					Services:  newServSet(),
				}
				sourceComp := &components.Component{
					Name:      "comp1",
					Resources: newResSet(),
					Services:  newServSet(),
				}
				mhm.EXPECT().GetComponent("comp1").Return(hostComp, nil)
				hostManifest := &manifests.ComponentManifest{}
				mhm.EXPECT().GetManifest(hostComp).Return(hostManifest, nil)

				msm.EXPECT().GetComponent("comp1").Return(sourceComp, nil)
				sourceManifest := &manifests.ComponentManifest{}
				msm.EXPECT().GetManifest(sourceComp).Return(sourceManifest, nil)
				vault.EXPECT().Lookup(mock.Anything, attributes.AttributesFilter{
					Hostname:  "localhost",
					Component: "comp1",
				}).Return(map[string]any{})
			},
			expectedError: false,
			validateGraph: func(t *testing.T, graph *ComponentGraph) {
				assert.Len(t, graph.List(), 1)
				tree, err := graph.Get("comp1")
				require.NoError(t, err)
				assert.NotNil(t, tree.host)
				assert.NotNil(t, tree.source)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mhm := NewMockHostManager(t)
			msm := NewMockSourceManager(t)
			mv := NewMockAttributesEngine(t)

			m := &Materia{
				Host:   mhm,
				Source: msm,
				Vault:  mv,
				Manifest: &manifests.MateriaManifest{
					Hosts: map[string]manifests.Host{
						"localhost": {},
					},
				},
			}

			mhm.EXPECT().GetHostname().Return("localhost")
			tt.setup(mhm, msm, mv)

			graph, err := m.BuildComponentGraph(context.Background(), tt.installedComps, tt.assignedComps)

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
		expectedPlan  []Action
		validatePlan  func(*testing.T, []Action)
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
			expectedPlan: []Action{
				{
					Todo:   ActionInstall,
					Target: components.Resource{Path: "hello"},
				},
				{
					Todo:   ActionInstall,
					Target: components.Resource{Path: "hello.container"},
				},
				{
					Todo:   ActionInstall,
					Target: components.Resource{Path: manifests.ComponentManifestFile},
				},
			},
		},
		{
			name:          "happy-path/secrets",
			component:     testComponents[5],
			expectedError: false,
			expectedPlan: []Action{
				{
					Todo:   ActionInstall,
					Target: components.Resource{Path: "hello-secret"},
				},
				{
					Todo:   ActionInstall,
					Target: components.Resource{Path: "hello.container"},
				},

				{
					Todo:   ActionInstall,
					Target: components.Resource{Path: manifests.ComponentManifestFile},
				},
				{
					Todo:   ActionInstall,
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
		expectedPlan  []Action
		setup         func(comp *components.Component, mhm *MockHostManager)
		validatePlan  func(*testing.T, []Action)
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
				Services: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.service",
					Static:  false,
				}),
			},
			expectedError: false,
			expectedPlan: []Action{
				{
					Todo:   ActionRemove,
					Target: components.Resource{Path: "hello.container"},
				},
				{
					Todo:   ActionRemove,
					Target: components.Resource{Path: manifests.ComponentManifestFile},
				},
				{
					Todo:   ActionRemove,
					Target: components.Resource{Path: "hello", Kind: components.ResourceTypeComponent},
				},
			},
		},
		{
			name: "happy-path/secrets",
			component: &components.Component{
				Name:      "hello-secret",
				State:     components.StateNeedRemoval,
				Resources: newResSet(testResources[0], testResources[6], testResources[7]),
				Services:  newServSet(),
			},
			expectedError: false,
			expectedPlan: []Action{
				{
					Todo:   ActionRemove,
					Target: components.Resource{Path: "hello.container"},
				},
				{
					Todo:   ActionRemove,
					Target: components.Resource{Path: "secret"},
				},
				{
					Todo:   ActionRemove,
					Target: components.Resource{Path: manifests.ComponentManifestFile},
				},
				{
					Todo:   ActionRemove,
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
				Services: newServSet(),
			},
			setup: func(comp *components.Component, mhm *MockHostManager) {
				mhm.EXPECT().GetVolume(mock.Anything, "systemd-hello").Return(&containers.Volume{
					Name: "systemd-hello",
				}, nil)
			},
			expectedError: false,
			opts:          PlannerConfig{CleanupQuadlets: true, CleanupVolumes: true},
			expectedPlan: []Action{
				{
					Todo:   ActionRemove,
					Target: components.Resource{Path: "hello.volume"},
				},
				{
					Todo:   ActionCleanup,
					Target: components.Resource{Path: "hello.volume"},
				},
				{
					Todo:   ActionRemove,
					Target: components.Resource{Path: "hello.container"},
				},
				{
					Todo:   ActionRemove,
					Target: components.Resource{Path: manifests.ComponentManifestFile},
				},
				{
					Todo:   ActionRemove,
					Target: components.Resource{Path: "hello", Kind: components.ResourceTypeComponent},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := NewMockHostManager(t)
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

func TestGenerateUpdatedComponentResources(t *testing.T) {
	tests := []struct {
		name         string
		stale, fresh *components.Component
		opts         PlannerConfig
		setup        func(host *MockHostManager, stale, fresh *components.Component)
		want         []Action
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
				Services: newServSet(),
			},
			fresh: &components.Component{
				Name:  "hello",
				State: components.StateFresh,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=hello"),
				),
				Services: newServSet(),
			},
			setup: func(host *MockHostManager, stale *components.Component, fresh *components.Component) {
			},
			want:    []Action{},
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
				Services: newServSet(),
			},
			fresh: &components.Component{
				Name:  "hello",
				State: components.StateFresh,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=goodbye"),
				),
				Services: newServSet(),
			},
			setup: func(host *MockHostManager, stale *components.Component, fresh *components.Component) {
			},
			want: []Action{
				planHelper(ActionUpdate, "hello", "hello.container"),
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
				Services: newServSet(),
			},
			fresh: &components.Component{
				Name:  "hello",
				State: components.StateFresh,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
				),
				Services: newServSet(),
			},
			setup: func(host *MockHostManager, stale *components.Component, fresh *components.Component) {
			},
			want: []Action{
				planHelper(ActionRemove, "hello", "hello.container"),
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
				Services: newServSet(),
			},
			fresh: &components.Component{
				Name:  "hello",
				State: components.StateFresh,
				Resources: newResSet(
					resourceHelper("MANIFEST.toml", "hello", ""),
					resourceHelper("hello.container", "hello", "[Container]\nImage=goodbye"),
				),
				Services: newServSet(),
			},
			setup: func(host *MockHostManager, stale *components.Component, fresh *components.Component) {
			},
			want: []Action{
				planHelper(ActionInstall, "hello", "hello.container"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := NewMockHostManager(t)
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

func TestProcessFreshComponentServices(t *testing.T) {
	tests := []struct {
		name         string
		component    *components.Component
		setup        func(comp *components.Component, sm *MockHostManager)
		want         []Action
		wantErr      bool
		validatePlan func(*testing.T, []Action)
	}{
		{
			name:      "no services",
			component: testComponents[0],
			want:      []Action{},
			setup:     func(comp *components.Component, services *MockHostManager) {},
		},
		{
			name:      "services - none running",
			component: testComponents[1],
			want: []Action{
				{
					Todo: ActionStart,
					Target: components.Resource{
						Path: "hello.service",
					},
				},
			},
			setup: func(comp *components.Component, sm *MockHostManager) {
				for _, src := range comp.Services.List() {
					sm.EXPECT().Get(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   "inactive",
						Enabled: false,
					}, nil)
				}
			},
		},
		{
			name:      "services - running",
			component: testComponents[1],
			setup: func(comp *components.Component, sm *MockHostManager) {
				for _, src := range comp.Services.List() {
					sm.EXPECT().Get(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   "active",
						Enabled: false,
					}, nil)
				}
			},
		},
		{
			name: "services - static",
			component: &components.Component{
				Name:      "hello",
				State:     components.StateFresh,
				Resources: newResSet(testResources[0]),
				Services: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.service",
					Static:  true,
				}),
			},
			want: []Action{
				{
					Todo: ActionEnable,
					Target: components.Resource{
						Path: "hello.service",
					},
				},
				{
					Todo: ActionStart,
					Target: components.Resource{
						Path: "hello.service",
					},
				},
			},
			setup: func(comp *components.Component, sm *MockHostManager) {
				for _, src := range comp.Services.List() {
					sm.EXPECT().Get(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   "inactive",
						Enabled: false,
					}, nil)
				}
			},
		},
		{
			name: "services - stopped",
			component: &components.Component{
				Name:      "hello",
				State:     components.StateFresh,
				Resources: newResSet(testResources[0]),
				Services: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.service",
					Stopped: true,
				}),
			},
			want: []Action{},
			setup: func(comp *components.Component, sm *MockHostManager) {
			},
		},
		{
			name: "services - container",
			component: &components.Component{
				Name:      "hello",
				State:     components.StateFresh,
				Resources: newResSet(testResources[0]),
				Services: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.container",
				}),
			},
			want: []Action{
				{
					Todo: ActionStart,
					Target: components.Resource{
						Path: "hello.container",
					},
				},
			},
			setup: func(parent *components.Component, sm *MockHostManager) {
				sm.EXPECT().Get(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: "inactive",
				}, nil)
			},
		},
		{
			name: "services - container with image",
			component: &components.Component{
				Name:      "hello",
				State:     components.StateFresh,
				Resources: newResSet(testResources[8]),
				Services: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.container",
				}),
			},
			want: []Action{
				{
					Todo: ActionStart,
					Target: components.Resource{
						Path: "hello.container",
					},
				},
			},
			setup: func(parent *components.Component, sm *MockHostManager) {
				sm.EXPECT().Get(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: "inactive",
				}, nil)
			},
			validatePlan: func(t *testing.T, a []Action) {
				assert.Equal(t, 1, len(a))
				assert.NotNil(t, a[0].Metadata)
				assert.Equal(t, *a[0].Metadata.ServiceTimeout, 60)
			},
		},
		{
			name: "services - container with defined image",
			component: &components.Component{
				Name:      "hello",
				State:     components.StateFresh,
				Resources: newResSet(testResources[8]),
				Services: newServSet(
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
			want: []Action{
				{
					Todo: ActionStart,
					Target: components.Resource{
						Path: "hello.container",
					},
				},
			},
			setup: func(parent *components.Component, sm *MockHostManager) {
				sm.EXPECT().Get(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: "inactive",
				}, nil)
			},
			validatePlan: func(t *testing.T, a []Action) {
				assert.Equal(t, 1, len(a))
				assert.NotNil(t, a[0].Metadata)
				assert.Equal(t, *a[0].Metadata.ServiceTimeout, 160)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := NewMockHostManager(t)
			tt.setup(tt.component, hm)
			got, gotErr := processFreshOrUnchangedComponentServices(context.Background(), hm, tt.component)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("processFreshComponentServices() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("processFreshComponentServices() succeeded unexpectedly")
			}
			for k, v := range tt.want {
				if k >= len(got) {
					t.Logf("Steps got: %v", got)
					t.Errorf("Missing step #%v: %v", k, v)
				}
				assert.Equal(t, v.Todo, got[k].Todo)
				assert.Equal(t, v.Target.Path, got[k].Target.Path)
			}
			if tt.validatePlan != nil {
				tt.validatePlan(t, got)
			}
		})
	}
}

func TestProcessRemovedComponentServices(t *testing.T) {
	tests := []struct {
		name         string
		component    *components.Component
		setup        func(comp *components.Component, sm *MockHostManager)
		want         []Action
		wantErr      bool
		validatePlan func(*testing.T, []Action)
	}{
		{
			name:      "no services",
			component: testComponents[0],
			want:      []Action{},
			setup:     func(comp *components.Component, services *MockHostManager) {},
		},
		{
			name:      "services - none running",
			component: testComponents[1],
			setup: func(comp *components.Component, sm *MockHostManager) {
				for _, src := range comp.Services.List() {
					sm.EXPECT().Get(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   "inactive",
						Enabled: false,
					}, nil)
				}
			},
		},
		{
			name:      "services - running",
			component: testComponents[1],
			want: []Action{
				{
					Todo: ActionStop,
					Target: components.Resource{
						Path: "hello.service",
					},
				},
			},
			setup: func(comp *components.Component, sm *MockHostManager) {
				for _, src := range comp.Services.List() {
					sm.EXPECT().Get(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   "active",
						Enabled: false,
					}, nil)
				}
			},
		},
		{
			name: "services - static",
			component: &components.Component{
				Name:      "hello",
				State:     components.StateNeedRemoval,
				Resources: newResSet(testResources[0]),
				Services: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.service",
					Static:  true,
				}),
			},
			want: []Action{
				{
					Todo: ActionStop,
					Target: components.Resource{
						Path: "hello.service",
					},
				},
			},
			setup: func(comp *components.Component, sm *MockHostManager) {
				for _, src := range comp.Services.List() {
					sm.EXPECT().Get(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   "active",
						Enabled: false,
					}, nil)
				}
			},
		},
		{
			name: "services - stopped",
			component: &components.Component{
				Name:      "hello",
				State:     components.StateNeedRemoval,
				Resources: newResSet(testResources[0]),
				Services: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.service",
					Stopped: true,
				}),
			},
			want: []Action{},
			setup: func(comp *components.Component, sm *MockHostManager) {
				for _, src := range comp.Services.List() {
					sm.EXPECT().Get(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   "inactive",
						Enabled: false,
					}, nil)
				}
			},
		},
		{
			name: "services - container",
			component: &components.Component{
				Name:      "hello",
				State:     components.StateNeedRemoval,
				Resources: newResSet(testResources[0]),
				Services: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.container",
				}),
			},
			want: []Action{
				{
					Todo: ActionStop,
					Target: components.Resource{
						Path: "hello.container",
					},
				},
			},
			setup: func(parent *components.Component, sm *MockHostManager) {
				sm.EXPECT().Get(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: "active",
				}, nil)
			},
		},
		{
			name: "services - container with image",
			component: &components.Component{
				Name:      "hello",
				State:     components.StateNeedRemoval,
				Resources: newResSet(testResources[8]),
				Services: newServSet(manifests.ServiceResourceConfig{
					Service: "hello.container",
				}),
			},
			want: []Action{
				{
					Todo: ActionStop,
					Target: components.Resource{
						Path: "hello.container",
					},
				},
			},
			setup: func(parent *components.Component, sm *MockHostManager) {
				sm.EXPECT().Get(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: "active",
				}, nil)
			},
			validatePlan: func(t *testing.T, a []Action) {
				assert.Equal(t, 1, len(a))
				assert.NotNil(t, a[0].Metadata)
				assert.Equal(t, *a[0].Metadata.ServiceTimeout, 60)
			},
		},
		{
			name: "services - container with defined image",
			component: &components.Component{
				Name:      "hello",
				State:     components.StateNeedRemoval,
				Resources: newResSet(testResources[8]),
				Services: newServSet(
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
			want: []Action{
				{
					Todo: ActionStop,
					Target: components.Resource{
						Path: "hello.container",
					},
				},
			},
			setup: func(parent *components.Component, sm *MockHostManager) {
				sm.EXPECT().Get(mock.Anything, "hello.image").Return(&services.Service{
					Name:  "hello-image.service",
					State: "inactive",
				}, nil)
				sm.EXPECT().Get(mock.Anything, "hello.service").Return(&services.Service{
					Name:  "hello.service",
					State: "active",
				}, nil)
			},
			validatePlan: func(t *testing.T, a []Action) {
				assert.Equal(t, 1, len(a))
				assert.NotNil(t, a[0].Metadata)
				assert.Equal(t, *a[0].Metadata.ServiceTimeout, 160)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := NewMockHostManager(t)
			tt.setup(tt.component, hm)
			got, gotErr := processRemovedComponentServices(context.Background(), hm, tt.component)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("processRemovedComponentServices() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("processRemovedComponentServices() succeeded unexpectedly")
			}
			for k, v := range tt.want {
				if k >= len(got) {
					t.Logf("Steps got: %v", got)
					t.Errorf("Missing step #%v: %v", k, v)
				}
				assert.Equal(t, v.Todo, got[k].Todo)
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
	result.Kind = components.NewComponent(parent).FindResourceType(result.Path)
	result.Content = content
	if result.Kind != components.ResourceTypeImage && result.Kind != components.ResourceTypeBuild {
		result.HostObject = fmt.Sprintf("systemd-%v", strings.TrimSuffix(filepath.Base(result.Path), filepath.Ext(result.Path)))
	}
	return result
}

func TestPlan(t *testing.T) {
	expected := []Action{
		planHelper(ActionInstall, "hello", ""),
		planHelper(ActionInstall, "hello", "hello.container"),
		planHelper(ActionInstall, "hello", "hello.env"),
		planHelper(ActionInstall, "hello", manifests.ComponentManifestFile),
		planHelper(ActionReload, "", ""),
	}
	man := &manifests.MateriaManifest{
		Hosts: map[string]manifests.Host{
			"localhost": {
				Components: []string{"hello"},
			},
		},
	}
	ctx := context.Background()
	sm := NewMockSourceManager(t)
	hm := NewMockHostManager(t)
	v := NewMockAttributesEngine(t)
	m := &Materia{Manifest: man, Source: sm, Host: hm, Vault: v, macros: testMacroMap}
	hm.EXPECT().GetHostname().Return("localhost")
	hm.EXPECT().ListInstalledComponents().Return([]string{}, nil)
	hm.EXPECT().ListVolumes(ctx).Return([]*containers.Volume{}, nil)
	containerResource := components.Resource{
		Path: "hello.container",
		Kind: components.ResourceTypeContainer,
	}
	dataResource := components.Resource{
		Path: "hello.env",
		Kind: components.ResourceTypeFile,
	}
	manifestResource := components.Resource{
		Path: manifests.MateriaManifestFile,
		Kind: components.ResourceTypeManifest,
	}
	helloComp := &components.Component{
		Name:      "hello",
		Resources: newResSet(containerResource, dataResource, manifestResource),
		State:     components.StateFresh,
		Defaults:  map[string]any{},
		Services:  newServSet(),
		Version:   components.DefaultComponentVersion,
	}
	sm.EXPECT().GetComponent("hello").Return(helloComp, nil)
	sm.EXPECT().GetManifest(helloComp).Return(&manifests.ComponentManifest{}, nil)
	v.EXPECT().Lookup(ctx, attributes.AttributesFilter{
		Hostname:  "localhost",
		Roles:     []string(nil),
		Component: "hello",
	}).Return(map[string]any{})
	sm.EXPECT().ReadResource(containerResource).Return("[Container]", nil)
	sm.EXPECT().ReadResource(dataResource).Return("FOO=bar", nil)
	sm.EXPECT().ReadResource(manifestResource).Return("", nil)

	plan, err := m.Plan(ctx)
	assert.NoError(t, err)
	for k, v := range plan.Steps() {
		expected := expected[k]
		assert.Equal(t, expected.Todo, v.Todo, "%v Todo not equal: %v != %v", v, v.Todo, expected.Todo)
		assert.Equal(t, expected.Parent.Name, v.Parent.Name, "Res %v Parent not equal: %v != %v", v.Target.Path, v.Parent.Name, expected.Parent.Name)
		assert.Equal(t, expected.Target.Path, v.Target.Path, "Res %v Path not equal: %v != %v", v.Target.Path, v.Target.Path, expected.Target.Path)
	}
}

func planHelper(todo ActionType, name, res string) Action {
	if res == "" {
		if name == "" {
			name = "root"
		}
		if todo == ActionReload {
			return Action{
				Todo:   todo,
				Parent: &components.Component{Name: name},
				Target: components.Resource{
					Parent: name,
					Kind:   components.ResourceTypeHost,
				},
			}
		}
		return Action{
			Todo:   todo,
			Parent: &components.Component{Name: name},
			Target: components.Resource{
				Parent: name,
				Kind:   components.ResourceTypeComponent,
				Path:   name,
			},
		}
	}
	act := Action{
		Todo: todo,
		Parent: &components.Component{
			Name: name,
		},
		Target: components.Resource{
			Parent: name,
			Kind:   components.NewComponent(name).FindResourceType(res),
			Path:   res,
		},
	}
	return act
}
