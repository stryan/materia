package executor

import (
	"context"

	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/containers"
)

type ContainerManager interface {
	ListContainers(context.Context, containers.ContainerListFilter) ([]*containers.Container, error)
	ExecContainer(context.Context, string, ...string) error

	DumpVolume(context.Context, *containers.Volume, string) error
	ImportVolume(context.Context, *containers.Volume, string) error
	RemoveVolume(context.Context, *containers.Volume) error

	GetNetwork(context.Context, string) (*containers.Network, error)
	RemoveNetwork(context.Context, *containers.Network) error

	WriteSecret(context.Context, string, string) error
	RemoveSecret(context.Context, string) error

	RemoveImage(context.Context, string) error
}

type Host interface {
	ServiceManager
	ContainerManager

	components.ComponentWriter
	InstallScript(context.Context, string, []byte) error
	RemoveScript(context.Context, string) error
	InstallUnit(context.Context, string, []byte) error
	RemoveUnit(context.Context, string) error
}
