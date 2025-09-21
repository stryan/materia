package materia

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"text/template"

	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/manifests"
	"primamateria.systems/materia/internal/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var testComponents = []*components.Component{
	{
		Name:      "hello",
		State:     components.StateFresh,
		Resources: []components.Resource{testResources[0]},
	},
	{
		Name:      "hello",
		State:     components.StateFresh,
		Resources: []components.Resource{testResources[0]},
		ServiceResources: map[string]manifests.ServiceResourceConfig{
			"hello.service": {
				Service: "hello.service",
				Static:  false,
			},
		},
	},
	{
		Name:      "hello",
		State:     components.StateFresh,
		Resources: []components.Resource{testResources[0], testResources[1], testResources[2], testResources[5]},
		ServiceResources: map[string]manifests.ServiceResourceConfig{
			"hello.service": {
				Service: "hello.service",
				Static:  false,
			},
		},
	},
	{
		Name:      "oldhello",
		State:     components.StateStale,
		Resources: []components.Resource{testResources[0]},
	},
	{
		Name:      "updated",
		State:     components.StateMayNeedUpdate,
		Resources: []components.Resource{testResources[0], testResources[3]},
		ServiceResources: map[string]manifests.ServiceResourceConfig{
			"hello.service": {
				Service: "hello.service",
				Static:  false,
			},
		},
	},
}

var testResources = []components.Resource{
	{
		Path:     "hello.container",
		Parent:   "hello",
		Kind:     components.ResourceTypeContainer,
		Template: true,
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
		"m_deps": func(arg string) (string, error) {
			switch arg {
			case "after":
				if res, ok := vars["After"]; ok {
					return res.(string), nil
				} else {
					return "local-fs.target network.target", nil
				}
			case "wants":
				if res, ok := vars["Wants"]; ok {
					return res.(string), nil
				} else {
					return "local-fs.target network.target", nil
				}
			case "requires":
				if res, ok := vars["Requires"]; ok {
					return res.(string), nil
				} else {
					return "local-fs.target network.target", nil
				}
			default:
				return "", errors.New("err bad default")
			}
		},
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

// TODO add newComponent,removeComponent tests

func TestMateria_updateComponents(t *testing.T) {
	tests := []struct {
		name                string
		assignedComponents  map[string]*components.Component
		installedComponents map[string]*components.Component
		expectedDiffs       map[string]*components.Component
		expectedError       string
	}{
		{
			name: "new component - not installed",
			assignedComponents: map[string]*components.Component{
				"comp1": {Name: "comp1", State: components.StateFresh},
			},
			installedComponents: map[string]*components.Component{},
			expectedDiffs: map[string]*components.Component{
				"comp1": {Name: "comp1", State: components.StateFresh},
			},
		},
		{
			name: "existing component - needs update",
			assignedComponents: map[string]*components.Component{
				"comp1": {Name: "comp1", State: components.StateFresh},
			},
			installedComponents: map[string]*components.Component{
				"comp1": {Name: "comp1", State: components.StateStale},
			},
			expectedDiffs: map[string]*components.Component{
				"comp1": {Name: "comp1", State: components.StateMayNeedUpdate},
			},
		},
		{
			name:               "stale component - needs removal",
			assignedComponents: map[string]*components.Component{},
			installedComponents: map[string]*components.Component{
				"comp1": {Name: "comp1", State: components.StateStale},
			},
			expectedDiffs: map[string]*components.Component{
				"comp1": {Name: "comp1", State: components.StateNeedRemoval},
			},
		},
		{
			name: "mixed scenario - new, existing, and stale components",
			assignedComponents: map[string]*components.Component{
				"comp1": {Name: "comp1", State: components.StateFresh}, // new
				"comp2": {Name: "comp2", State: components.StateFresh}, // existing
			},
			installedComponents: map[string]*components.Component{
				"comp2": {Name: "comp2", State: components.StateStale}, // existing
				"comp3": {Name: "comp3", State: components.StateStale}, // stale
			},
			expectedDiffs: map[string]*components.Component{
				"comp1": {Name: "comp1", State: components.StateFresh},         // new
				"comp2": {Name: "comp2", State: components.StateMayNeedUpdate}, // existing
				"comp3": {Name: "comp3", State: components.StateNeedRemoval},   // stale
			},
		},
		{
			name:               "installed component not stale - should error",
			assignedComponents: map[string]*components.Component{},
			installedComponents: map[string]*components.Component{
				"comp1": {Name: "comp1", State: components.StateUnknown}, // not stale
			},
			expectedDiffs: map[string]*components.Component{},
			expectedError: "installed component isn't stale",
		},
		{
			name: "assigned component not fresh - should error",
			assignedComponents: map[string]*components.Component{
				"comp1": {Name: "comp1", State: components.StateUnknown},
			},
			installedComponents: map[string]*components.Component{},
			expectedDiffs:       map[string]*components.Component{},
			expectedError:       "assigned component isn't fresh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDiffs, gotErr := updateComponents(tt.assignedComponents, tt.installedComponents)

			// Check error
			if tt.expectedError != "" {
				require.NotNil(t, gotErr)
				require.Contains(t, gotErr.Error(), tt.expectedError)
			} else {
				// Check diffs
				require.Equal(t, gotDiffs, tt.expectedDiffs, "updateComponents() gotDiffs = %v, expectedDiffs %v", gotDiffs, tt.expectedDiffs)
			}
		})
	}
}

func TestMateria_calculateFreshComponentResources(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		setup func(comp *components.Component, source *MockComponentRepository)
		// Named input parameters for target function.
		newComponent *components.Component
		vars         map[string]any
		want         []Action
		wantErr      bool
	}{
		{
			name:         "basic component",
			newComponent: testComponents[1],
			vars:         map[string]any{},
			want: []Action{
				{
					Todo: ActionInstall,
				},
				{
					Todo:    ActionInstall,
					Payload: components.Resource{Path: "hello.container"},
				},
			},
			setup: func(comp *components.Component, source *MockComponentRepository) {
				source.EXPECT().ReadResource(testResources[0]).Return("[Container]", nil)
			},
			wantErr: false,
		},
		{
			name:         "multi file component",
			newComponent: testComponents[2],
			vars:         map[string]any{},
			want: []Action{
				{
					Todo: ActionInstall,
				},
				{
					Todo:    ActionInstall,
					Payload: components.Resource{Path: "deep.env"},
				},
				{
					Todo:    ActionInstall,
					Payload: components.Resource{Path: "hello.container"},
				},
				{
					Todo:    ActionInstall,
					Payload: components.Resource{Path: "hello.env"},
				},
				{
					Todo:    ActionInstall,
					Payload: components.Resource{Path: "hello.sh"},
				},
			},
			setup: func(comp *components.Component, source *MockComponentRepository) {
				source.EXPECT().ReadResource(testResources[5]).Return("inner file", nil)
				source.EXPECT().ReadResource(testResources[0]).Return("[Container]", nil)
				source.EXPECT().ReadResource(testResources[1]).Return("Hello env", nil)
				source.EXPECT().ReadResource(testResources[2]).Return("Hello service", nil)
			},
		},
		{
			name:         "not fresh",
			newComponent: testComponents[3],
			vars:         map[string]any{},
			wantErr:      true,
			setup: func(comp *components.Component, source *MockComponentRepository) {
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceRepo := NewMockComponentRepository(t)
			tt.setup(tt.newComponent, sourceRepo)
			m := &Materia{SourceRepo: sourceRepo, macros: testMacroMap}
			got, gotErr := m.calculateFreshComponentResources(tt.newComponent, tt.vars)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("calculateFreshComponent() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("calculateFreshComponent() succeeded unexpectedly")
			}
			for k, v := range tt.want {
				if k >= len(got) {
					t.Errorf("Missing step #%v: %v", k, v)
				}
				assert.Equal(t, v.Todo, tt.want[k].Todo)
				assert.Equal(t, v.Payload.Path, tt.want[k].Payload.Path)
			}
		})
	}
}

func TestMateria_calculateRemovedComponentResources(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		comp    *components.Component
		want    []Action
		wantErr bool
	}{
		{
			name: "basic removal",
			comp: &components.Component{
				Name:      "hello",
				State:     components.StateNeedRemoval,
				Resources: []components.Resource{testResources[0]},
			},
			want: []Action{
				{
					Todo:    ActionRemove,
					Payload: components.Resource{Path: "hello", Kind: components.ResourceTypeComponent},
				},
				{
					Todo:    ActionRemove,
					Payload: components.Resource{Path: "hello.container"},
				},
			},
			wantErr: false,
		},
		{
			name: "multi-file removal",
			comp: &components.Component{
				Name:      "hello",
				State:     components.StateNeedRemoval,
				Resources: []components.Resource{testResources[0], testResources[1], testResources[2], testResources[5]},
				ServiceResources: map[string]manifests.ServiceResourceConfig{
					"hello.service": {
						Service: "hello.service",
						Static:  false,
					},
				},
			},
			want: []Action{
				{
					Todo:    ActionRemove,
					Payload: components.Resource{Path: "deep.env"},
				},
				{
					Todo:    ActionRemove,
					Payload: components.Resource{Path: "hello.container"},
				},
				{
					Todo:    ActionRemove,
					Payload: components.Resource{Path: "hello.env"},
				},
				{
					Todo:    ActionRemove,
					Payload: components.Resource{Path: "hello.sh"},
				},
				{
					Todo:    ActionRemove,
					Payload: components.Resource{Path: "hello", Kind: components.ResourceTypeComponent},
				},
			},
			wantErr: false,
		},
		{
			name:    "not to be removed",
			comp:    testComponents[0],
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Materia{}
			got, gotErr := m.calculateRemovedComponentResources(tt.comp)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("calculateRemovedComponentResources() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("calculateRemovedComponentResources() succeeded unexpectedly")
			}
			for k, v := range tt.want {
				if k >= len(got) {
					t.Log(got)
					t.Errorf("Missing step #%v: %v", k, v)
				}
				assert.Equal(t, v.Todo, tt.want[k].Todo)
				assert.Equal(t, v.Payload.Path, tt.want[k].Payload.Path)
			}
		})
	}
}

func TestMateria_processFreshComponentServices(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		component *components.Component
		setup     func(comp *components.Component, sm *MockServices)
		want      []Action
		wantErr   bool
	}{
		{
			name:      "no services",
			component: testComponents[0],
			want:      []Action{},
			setup:     func(comp *components.Component, services *MockServices) {},
		},
		{
			name:      "services - none running",
			component: testComponents[1],
			want: []Action{
				{
					Todo: ActionStart,
					Payload: components.Resource{
						Path: "hello.service",
					},
				},
			},
			setup: func(comp *components.Component, sm *MockServices) {
				for _, src := range comp.ServiceResources {
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
			setup: func(comp *components.Component, sm *MockServices) {
				for _, src := range comp.ServiceResources {
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
				Resources: []components.Resource{testResources[0]},
				ServiceResources: map[string]manifests.ServiceResourceConfig{
					"hello.service": {
						Service: "hello.service",
						Static:  true,
					},
				},
			},
			want: []Action{
				{
					Todo: ActionEnable,
					Payload: components.Resource{
						Path: "hello.service",
					},
				},
				{
					Todo: ActionStart,
					Payload: components.Resource{
						Path: "hello.service",
					},
				},
			},
			setup: func(comp *components.Component, sm *MockServices) {
				for _, src := range comp.ServiceResources {
					sm.EXPECT().Get(mock.Anything, src.Service).Return(&services.Service{
						Name:    src.Service,
						State:   "inactive",
						Enabled: false,
					}, nil)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := NewMockServices(t)
			m := &Materia{Services: ms}
			tt.setup(tt.component, ms)
			got, gotErr := m.processFreshComponentServices(context.Background(), tt.component)
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
					t.Log(got)
					t.Errorf("Missing step #%v: %v", k, v)
				}
				assert.Equal(t, v.Todo, tt.want[k].Todo)
				assert.Equal(t, v.Payload.Path, tt.want[k].Payload.Path)
			}
		})
	}
}

func TestMateria_diffComponent(t *testing.T) {
	tests := []struct {
		name         string // description of this test case
		original     *components.Component
		newComponent *components.Component
		setup        func(oldc, newc *components.Component, source *MockComponentRepository, host *MockComponentRepository)
		vars         map[string]any
		want         []Action
		wantErr      bool
	}{
		{
			name: "simple update",
			original: &components.Component{
				Name:      "updated",
				State:     components.StateStale,
				Resources: []components.Resource{testResources[0], testResources[3]},
				ServiceResources: map[string]manifests.ServiceResourceConfig{
					"hello.service": {
						Service: "hello.service",
						Static:  false,
					},
				},
			},
			newComponent: testComponents[4],
			setup: func(oldc, newc *components.Component, source *MockComponentRepository, host *MockComponentRepository) {
				host.EXPECT().ReadResource(oldc.Resources[0]).Return("container file!", nil)
				source.EXPECT().ReadResource(newc.Resources[0]).Return("[Container]\nImage=ubi8", nil)
				host.EXPECT().ReadResource(oldc.Resources[1]).Return("manifestation", nil)
				source.EXPECT().ReadResource(newc.Resources[1]).Return("manifestation", nil)
			},
			want: []Action{
				{
					Todo:    ActionUpdate,
					Payload: components.Resource{Path: "hello.container"},
				},
			},
		},
		{
			name: "defaults update",
			original: &components.Component{
				Name:      "updated",
				State:     components.StateStale,
				Defaults:  map[string]any{"var": "hello"},
				Resources: []components.Resource{testResources[0], testResources[3]},
				ServiceResources: map[string]manifests.ServiceResourceConfig{
					"hello.service": {
						Service: "hello.service",
						Static:  false,
					},
				},
			},
			newComponent: &components.Component{
				Name:      "updated",
				State:     components.StateMayNeedUpdate,
				Resources: []components.Resource{testResources[0], testResources[3]},
				Defaults:  map[string]any{"var": "goodbye"},
				ServiceResources: map[string]manifests.ServiceResourceConfig{
					"hello.service": {
						Service: "hello.service",
						Static:  false,
					},
				},
			},
			setup: func(oldc, newc *components.Component, source *MockComponentRepository, host *MockComponentRepository) {
				host.EXPECT().ReadResource(oldc.Resources[0]).Return("container hello", nil)
				source.EXPECT().ReadResource(newc.Resources[0]).Return("[Container]\nImage={{ .var }}", nil)
				host.EXPECT().ReadResource(oldc.Resources[1]).Return("manifestation", nil)
				source.EXPECT().ReadResource(newc.Resources[1]).Return("manifestation", nil)
			},
			want: []Action{
				{
					Todo:    ActionUpdate,
					Payload: components.Resource{Path: "hello.container"},
				},
			},
		},
		{
			name: "file removed",
			original: &components.Component{
				Name:      "updated",
				State:     components.StateStale,
				Resources: []components.Resource{testResources[0], testResources[3]},
				ServiceResources: map[string]manifests.ServiceResourceConfig{
					"hello.service": {
						Service: "hello.service",
						Static:  false,
					},
				},
			},
			newComponent: &components.Component{
				Name:      "updated",
				State:     components.StateMayNeedUpdate,
				Resources: []components.Resource{testResources[3]},
			},
			setup: func(oldc, newc *components.Component, source *MockComponentRepository, host *MockComponentRepository) {
				host.EXPECT().ReadResource(oldc.Resources[1]).Return("manifestation", nil)
				source.EXPECT().ReadResource(newc.Resources[0]).Return("manifestation", nil)
			},
			want: []Action{
				{
					Todo:    ActionRemove,
					Payload: components.Resource{Path: "hello.container"},
				},
			},
		},
		{
			name: "file renamed",
			original: &components.Component{
				Name:      "updated",
				State:     components.StateStale,
				Resources: []components.Resource{testResources[0], testResources[3]},
				ServiceResources: map[string]manifests.ServiceResourceConfig{
					"hello.service": {
						Service: "hello.service",
						Static:  false,
					},
				},
			},
			newComponent: &components.Component{
				Name:      "updated",
				State:     components.StateMayNeedUpdate,
				Resources: []components.Resource{testResources[4], testResources[3]},
			},
			setup: func(oldc, newc *components.Component, source *MockComponentRepository, host *MockComponentRepository) {
				host.EXPECT().ReadResource(oldc.Resources[1]).Return("manifestation", nil)
				source.EXPECT().ReadResource(newc.Resources[0]).Return("[Container]", nil)
				source.EXPECT().ReadResource(newc.Resources[1]).Return("manifestation", nil)
			},
			want: []Action{
				{
					Todo:    ActionRemove,
					Payload: components.Resource{Path: "hello.container"},
				},
				{
					Todo:    ActionInstall,
					Payload: components.Resource{Path: "goodbye.container"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceRepo := NewMockComponentRepository(t)
			hostrepo := NewMockComponentRepository(t)
			m := &Materia{SourceRepo: sourceRepo, CompRepo: hostrepo, macros: testMacroMap}
			tt.setup(tt.original, tt.newComponent, sourceRepo, hostrepo)
			got, gotErr := m.diffComponent(tt.original, tt.newComponent, tt.vars)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("diffComponent() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("diffComponent() succeeded unexpectedly")
			}
			for k, v := range tt.want {
				if k >= len(got) {
					t.Log(got)
					t.Errorf("Missing step #%v: %v", k, v)
				}
				assert.Equal(t, v.Todo, tt.want[k].Todo)
				assert.Equal(t, v.Payload.Path, tt.want[k].Payload.Path)
			}
		})
	}
}
