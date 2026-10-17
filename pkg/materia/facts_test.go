package materia

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/facts"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/mocks"
	"primamateria.systems/materia/pkg/repository"
)

func fakeOutput(t *testing.T, input executionOutput, out *repository.ManagedDir, index int) {
	t.Helper()
	content := bytes.NewBuffer([]byte{})
	require.Nil(t, toml.NewEncoder(content).Encode(input))
	name := "lastrun.toml"
	if index != 0 {
		name = fmt.Sprintf("lastrun-%v.toml", index)
	}
	require.Nil(t, out.Install(t.Context(), name, content.Bytes()))
}

func TestMateriaCollector_Collect(t *testing.T) {
	presentday := time.Date(1998, time.July, 6, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		accum   *facts.Facts
		setup   func() MateriaCollector
		want    MateriaFacts
		wantErr bool
	}{
		{
			name: "invalid",
			setup: func() MateriaCollector {
				return MateriaCollector{
					hostname: "ivy",
				}
			},
			wantErr: true,
		},
		{
			name: "empty",
			setup: func() MateriaCollector {
				lister := mocks.NewMockHostManager(t)
				lister.EXPECT().ListInstalledComponents().Return([]string{}, nil)
				return MateriaCollector{
					hostname:   "ivy",
					config:     &MateriaConfig{},
					manifest:   &manifests.MateriaManifest{},
					hostLister: lister,
				}
			},
			want:    MateriaFacts{Hostname: "ivy", InstalledComponents: []string{}, AssignedComponents: []string{}},
			wantErr: false,
		},
		{
			name: "basic",
			setup: func() MateriaCollector {
				lister := mocks.NewMockHostManager(t)
				lister.EXPECT().ListInstalledComponents().Return([]string{}, nil)
				return MateriaCollector{
					hostname: "ivy",
					config: &MateriaConfig{
						Roles: []string{"support"},
					},
					manifest: &manifests.MateriaManifest{
						Roles: map[string]manifests.Role{
							"support": {
								Components: []string{"healer"},
							},
						},
					},
					hostLister: lister,
				}
			},
			want:    MateriaFacts{Hostname: "ivy", Roles: []string{"support"}, InstalledComponents: []string{}, AssignedComponents: []string{"healer"}},
			wantErr: false,
		},
		{
			name: "assigned and installed",
			setup: func() MateriaCollector {
				lister := mocks.NewMockHostManager(t)
				lister.EXPECT().ListInstalledComponents().Return([]string{"gargoyle"}, nil)
				return MateriaCollector{
					hostname: "ivy",
					config:   &MateriaConfig{},
					manifest: &manifests.MateriaManifest{
						Hosts: map[string]manifests.Host{
							"ivy": {
								Components: []string{"gargoyle"},
								Roles:      []string{"support"},
							},
						},
						Roles: map[string]manifests.Role{
							"support": {
								Components: []string{"healer"},
							},
						},
					},
					hostLister: lister,
				}
			},
			want:    MateriaFacts{Hostname: "ivy", Roles: []string{"support"}, InstalledComponents: []string{"gargoyle"}, AssignedComponents: []string{"gargoyle", "healer"}},
			wantErr: false,
		},
		{
			name:  "output facts",
			accum: &facts.Facts{},
			setup: func() MateriaCollector {
				lister := mocks.NewMockHostManager(t)
				lister.EXPECT().ListInstalledComponents().Return([]string{}, nil)
				fakeOut, err := repository.NewManagedDir(t.TempDir(), 0o644)
				require.Nil(t, err)
				fakeOutput(t, executionOutput{
					EndRevision: "latest",
					Timestamp:   presentday,
				}, fakeOut, 0)
				return MateriaCollector{
					hostname:   "ivy",
					config:     &MateriaConfig{},
					manifest:   &manifests.MateriaManifest{},
					hostLister: lister,
					OutputDir:  fakeOut,
				}
			},
			want: MateriaFacts{
				Hostname:            "ivy",
				AssignedComponents:  []string{},
				InstalledComponents: []string{},
				LastRevision:        "latest",
				LastRun:             presentday.UTC().Format(time.RFC3339),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setup()
			if err := c.Validate(); err != nil {
				if tt.wantErr {
					return
				}
				require.Nil(t, err, "wanted a valid collector")
			}
			got, gotErr := c.Collect(t.Context(), tt.accum)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Collect() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Collect() succeeded unexpectedly")
			}
			mfacts, ok := got.(MateriaFacts)
			require.True(t, ok)
			require.EqualValues(t, tt.want, mfacts, "wanted %v got %v", tt.want, got)
		})
	}
}
