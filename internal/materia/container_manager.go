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
	DumpVolume(context.Context, *containers.Volume, string, bool) error
	ImportVolume(context.Context, *containers.Volume, string) error
	MountVolume(context.Context, *containers.Volume) error
	ListSecrets(context.Context) ([]string, error)
	GetSecret(context.Context, string) (*containers.PodmanSecret, error)
	WriteSecret(context.Context, string, string) error
	RemoveSecret(context.Context, string) error
	SecretName(string) string
	Close()
}
