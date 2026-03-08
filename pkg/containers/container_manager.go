package containers

import (
	"context"
)

type ContainerManager interface {
	GetContainer(context.Context, string) (*Container, error)
	ListContainers(context.Context, ContainerListFilter) ([]*Container, error)
	PauseContainer(context.Context, string) error
	UnpauseContainer(context.Context, string) error
	ExecContainer(context.Context, string, string) error

	GetVolume(context.Context, string) (*Volume, error)
	ListVolumes(context.Context) ([]*Volume, error)
	DumpVolume(context.Context, *Volume, string) error
	ImportVolume(context.Context, *Volume, string) error
	MountVolume(context.Context, *Volume) error
	RemoveVolume(context.Context, *Volume) error

	ListNetworks(context.Context) ([]*Network, error)
	GetNetwork(context.Context, string) (*Network, error)
	RemoveNetwork(context.Context, *Network) error

	ListSecrets(context.Context) ([]string, error)
	GetSecret(context.Context, string) (*PodmanSecret, error)
	WriteSecret(context.Context, string, string) error
	RemoveSecret(context.Context, string) error
	SecretName(string) string

	ListImages(context.Context) ([]*Image, error)
	RemoveImage(context.Context, string) error
}
