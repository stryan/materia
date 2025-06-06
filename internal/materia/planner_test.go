package materia

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"
	"text/template"

	"git.saintnet.tech/stryan/materia/internal/components"
	"git.saintnet.tech/stryan/materia/internal/manifests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

var testComponents = map[string]*components.Component{
	"hello": {
		Name:      "hello",
		State:     components.StateFresh,
		Resources: []components.Resource{testResources[0]},
	},
	"hello-serv": {
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
}

var testResources = []components.Resource{
	{
		Name:     "hello.container",
		Parent:   "hello",
		Path:     "/hello.container.gotmpl",
		Kind:     components.ResourceTypeContainer,
		Template: true,
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

func TestMateria_calculateFreshComponent(t *testing.T) {
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
			newComponent: testComponents["hello-serv"],
			vars:         map[string]any{},
			want: []Action{
				{
					Todo: ActionInstallComponent,
				},
				{
					Todo:    ActionInstallQuadlet,
					Payload: components.Resource{Name: "hello.container"},
				},
				{
					Todo:    ActionInstallFile,
					Payload: components.Resource{Name: "MANIFEST.toml"},
				},
			},
			setup: func(comp *components.Component, source *MockComponentRepository) {
				source.EXPECT().ReadResource(testResources[0]).Return("Hello", nil)
			},
			wantErr: false,
		},
		{
			name:         "basic component - with services",
			newComponent: testComponents["hello"],
			vars:         map[string]any{},
			want: []Action{
				{
					Todo: ActionInstallComponent,
				},
				{
					Todo:    ActionInstallQuadlet,
					Payload: components.Resource{Name: "hello.container"},
				},
				{
					Todo:    ActionInstallFile,
					Payload: components.Resource{Name: "MANIFEST.toml"},
				},
				{
					Todo: ActionReloadUnits,
				},
				{
					Todo:    ActionStartService,
					Payload: components.Resource{Name: "hello.service"},
				},
			},
			setup: func(comp *components.Component, source *MockComponentRepository) {
				source.EXPECT().ReadResource(testResources[0]).Return("Hello", nil)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceRepo := NewMockComponentRepository(t)
			tt.setup(tt.newComponent, sourceRepo)
			got, gotErr := calculateFreshComponent(sourceRepo, tt.newComponent, tt.vars, testMacroMap)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("calculateFreshComponent() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("calculateFreshComponent() succeeded unexpectedly")
			}
			for k, v := range got {
				assert.Equal(t, v.Todo, tt.want[k].Todo)
				assert.Equal(t, v.Payload.Name, tt.want[k].Payload.Name)
			}
		})
	}
}
