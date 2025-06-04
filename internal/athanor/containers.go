package athanor

import (
	"context"

	"git.saintnet.tech/stryan/materia/internal/containers"
)

type ContainerManager interface {
	PauseContainer(context.Context, string) error
	UnpauseContainer(context.Context, string) error
	DumpVolume(context.Context, containers.Volume, string, bool) error
}
