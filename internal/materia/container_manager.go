package materia

import (
	"context"

	"primamateria.systems/materia/internal/containers"
)

type ContainerManager interface {
	GetVolume(context.Context, string) (*containers.Volume, error)
	ListVolumes(context.Context) ([]*containers.Volume, error)
	GetContainer(context.Context, string) (*containers.Container, error)
	ListContainers(context.Context, containers.ContainerListFilter) ([]*containers.Container, error)
	PauseContainer(context.Context, string) error
	UnpauseContainer(context.Context, string) error
	DumpVolume(context.Context, *containers.Volume, string, bool) error
	ImportVolume(context.Context, *containers.Volume, string) error
	MountVolume(context.Context, *containers.Volume) error
	RemoveVolume(context.Context, *containers.Volume) error
	ListNetworks(context.Context) ([]*containers.Network, error)
	GetNetwork(context.Context, string) (*containers.Network, error)
	RemoveNetwork(context.Context, *containers.Network) error
	ListSecrets(context.Context) ([]string, error)
	GetSecret(context.Context, string) (*containers.PodmanSecret, error)
	WriteSecret(context.Context, string, string) error
	RemoveSecret(context.Context, string) error
	SecretName(string) string
	ListImages(context.Context) ([]*containers.Image, error)
	RemoveImage(context.Context, string) error
}
