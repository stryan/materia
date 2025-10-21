package materia

import (
	"primamateria.systems/materia/pkg/manifests"
)

type SourceManager interface {
	ComponentRepository
	LoadManifest(string) (*manifests.MateriaManifest, error)
	AddSource(Source) error
}
