package materia

import (
	"context"

	"primamateria.systems/materia/pkg/manifests"
)

type SourceManager interface {
	ComponentReader
	LoadManifest(string) (*manifests.MateriaManifest, error)
	AddSource(Source) error
	Sync(context.Context) error
}
