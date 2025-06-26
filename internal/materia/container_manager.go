package materia

import (
	"context"

	"primamateria.systems/materia/internal/containers"
)

type ContainerManager interface {
	InspectVolume(string) (*containers.Volume, error)
	ListVolumes(context.Context) ([]*containers.Volume, error)
	PauseContainer(context.Context, string) error
	UnpauseContainer(context.Context, string) error
	DumpVolume(context.Context, containers.Volume, string, bool) error
	Close()
}
