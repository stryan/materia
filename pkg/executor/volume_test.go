package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/mocks"
	"primamateria.systems/materia/pkg/components"
)

func Test_CleanupVolume(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(host *mocks.MockHostManager, input components.Resource)
		input   components.Resource
		wantErr bool
	}{
		{
			name: "un-used volume",
			setup: func(hm *mocks.MockHostManager, input components.Resource) {
				hm.EXPECT().ListContainers(mock.Anything, containers.ContainerListFilter{
					Volume: "systemd-hello",
					All:    true,
				}).Return([]*containers.Container{}, nil)
				hm.EXPECT().RemoveVolume(mock.Anything, &containers.Volume{Name: "systemd-hello"}).Return(nil)
			},
			input: components.Resource{
				Path:       "hello.volume",
				HostObject: "systemd-hello",
				Kind:       components.ResourceTypeVolume,
			},
			wantErr: false,
		},
		{
			name: "in-use volume",
			setup: func(hm *mocks.MockHostManager, input components.Resource) {
				hm.EXPECT().ListContainers(mock.Anything, containers.ContainerListFilter{
					Volume: "systemd-hello",
					All:    true,
				}).Return([]*containers.Container{
					{
						Name: "hello",
					},
				}, nil)
			},
			input: components.Resource{
				Path:       "hello.volume",
				HostObject: "systemd-hello",
				Kind:       components.ResourceTypeVolume,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := mocks.NewMockHostManager(t)
			e := Executor{host: hm}
			tt.setup(hm, tt.input)
			action := actions.Action{
				Todo:   actions.ActionCleanup,
				Target: tt.input,
			}
			err := cleanupVolume(context.Background(), &e, action)
			if tt.wantErr {
				assert.Error(t, err, "wanted errr, got nil")
			} else {
				assert.Nil(t, err, "wanted nil, got err", err)
			}
		})
	}
}
