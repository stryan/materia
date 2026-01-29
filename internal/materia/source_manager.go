package materia

import (
	"context"

	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/source"
)

type SourceManager interface {
	components.ComponentReader
	LoadManifest(string) (*manifests.MateriaManifest, error)
	AddSource(source.Source) error
	Sync(context.Context) error
}
