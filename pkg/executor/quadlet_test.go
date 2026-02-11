package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/mocks"
	"primamateria.systems/materia/internal/services"
	"primamateria.systems/materia/pkg/components"
)

func Test_CleanupNetwork(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(host *mocks.MockHostManager, input components.Resource)
		input   components.Resource
		wantErr bool
	}{
		{
			name: "un-used network",
			setup: func(hm *mocks.MockHostManager, input components.Resource) {
				hm.EXPECT().GetNetwork(mock.Anything, input.HostObject).Return(&containers.Network{
					Name:       input.HostObject,
					Containers: []containers.NetworkContainer{},
				}, nil)
				hm.EXPECT().RemoveNetwork(mock.Anything, &containers.Network{Name: "test-network"}).Return(nil)
			},
			input: components.Resource{
				Path:       "test-network.network",
				HostObject: "test-network",
				Kind:       components.ResourceTypeNetwork,
			},
			wantErr: false,
		},
		{
			name: "in-use network",
			setup: func(hm *mocks.MockHostManager, input components.Resource) {
				hm.EXPECT().GetNetwork(mock.Anything, input.HostObject).Return(&containers.Network{
					Name: input.HostObject,
					Containers: []containers.NetworkContainer{
						{
							Name: "hello.container",
						},
					},
				}, nil)
			},
			input: components.Resource{
				Path:       "test-network.network",
				HostObject: "test-network",
				Kind:       components.ResourceTypeNetwork,
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
			err := cleanupNetwork(context.Background(), &e, action)
			if tt.wantErr {
				assert.Error(t, err, "wanted errr, got nil")
			} else {
				assert.Nil(t, err, "wanted nil, got err", err)
			}
		})
	}
}

func Test_CleanupBuildArtifact(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(host *mocks.MockHostManager, input components.Resource)
		input   components.Resource
		wantErr bool
	}{
		{
			name: "un-used image",
			setup: func(hm *mocks.MockHostManager, input components.Resource) {
				hm.EXPECT().ListContainers(mock.Anything, containers.ContainerListFilter{
					Image: "image:latest",
					All:   true,
				}).Return([]*containers.Container{}, nil)
				hm.EXPECT().RemoveImage(mock.Anything, "image:latest").Return(nil)
			},
			input: components.Resource{
				Path:       "test-image.image",
				Kind:       components.ResourceTypeImage,
				HostObject: "image:latest",
			},
			wantErr: false,
		},
		{
			name: "un-used build artifact",
			setup: func(hm *mocks.MockHostManager, input components.Resource) {
				hm.EXPECT().ListContainers(mock.Anything, containers.ContainerListFilter{
					Image: "image:latest",
					All:   true,
				}).Return([]*containers.Container{}, nil)
				hm.EXPECT().RemoveImage(mock.Anything, "image:latest").Return(nil)
			},
			input: components.Resource{
				Path:       "test-image.build",
				HostObject: "image:latest",
				Kind:       components.ResourceTypeBuild,
			},
			wantErr: false,
		},
		{
			name: "in-use image",
			setup: func(hm *mocks.MockHostManager, input components.Resource) {
				hm.EXPECT().ListContainers(mock.Anything, containers.ContainerListFilter{
					Image: "image:latest",
					All:   true,
				}).Return([]*containers.Container{
					{
						Name: "hello",
					},
				}, nil)
			},
			input: components.Resource{
				Path:       "test-image.image",
				Kind:       components.ResourceTypeImage,
				HostObject: "image:latest",
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
			err := cleanupBuildArtifact(context.Background(), &e, action)
			if tt.wantErr {
				assert.Error(t, err, "wanted errr, got nil")
			} else {
				assert.Nil(t, err, "wanted nil, got err", err)
			}
		})
	}
}

func TestEnsureQuadlet(t *testing.T) {
	ctx := context.Background()
	hm := mocks.NewMockHostManager(t)

	comp := &components.Component{
		Name: "hello",
	}

	resource := components.Resource{
		Path:   "hello.container",
		Parent: comp.Name,
		Kind:   components.ResourceTypeContainer,
	}

	action := actions.Action{
		Todo:   actions.ActionEnsure,
		Target: resource,
		Parent: comp,
	}

	e := &Executor{
		host:           hm,
		defaultTimeout: 30,
	}

	hm.EXPECT().Apply(ctx, "", services.ServiceReloadUnits, 30).Return(nil)
	hm.EXPECT().Apply(ctx, "hello.service", services.ServiceRestart, 30).Return(nil)

	assert.NoError(t, ensureQuadlet(ctx, e, action))
}
