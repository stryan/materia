package materia

import (
	"context"

	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/services"
)

type ServiceManager interface {
	ApplyService(context.Context, string, services.ServiceAction, int) error
	GetService(context.Context, string) (*services.Service, error)
	RunOneshotCommand(context.Context, int, string, []string) error
	WaitUntilState(context.Context, string, string, int) error
	Close() error
}

type ContainerManager interface {
	ListNetworks(context.Context) ([]*containers.Network, error)
	GetNetwork(context.Context, string) (*containers.Network, error)
	RemoveNetwork(context.Context, *containers.Network) error

	ListVolumes(context.Context) ([]*containers.Volume, error)
	ImportVolume(context.Context, *containers.Volume, string) error
	DumpVolume(context.Context, *containers.Volume, string) error
	GetVolume(context.Context, string) (*containers.Volume, error)
	RemoveVolume(context.Context, *containers.Volume) error

	ListImages(context.Context) ([]*containers.Image, error)
	RemoveImage(context.Context, string) error

	GetContainer(context.Context, string) (*containers.Container, error)
	ListContainers(context.Context, containers.ContainerListFilter) ([]*containers.Container, error)
	ExecContainer(context.Context, string, ...string) error

	ListSecrets(context.Context) ([]string, error)
	GetSecret(context.Context, string) (*containers.PodmanSecret, error)
	WriteSecret(context.Context, string, string) error
	RemoveSecret(context.Context, string) error
	SecretName(string) string
}

type HostManager interface {
	ContainerManager
	components.ComponentReader
	components.ComponentWriter
	FactsProvider
	ServiceManager
	ListInstalledComponents() ([]string, error)
	InstallScript(context.Context, string, []byte) error
	RemoveScript(context.Context, string) error
	InstallUnit(context.Context, string, []byte) error
	RemoveUnit(context.Context, string) error
}
