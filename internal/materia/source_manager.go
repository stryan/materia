package materia

import (
	"primamateria.systems/materia/internal/manifests"
)

type SourceManager interface {
	ComponentRepository
	LoadManifest(string) (*manifests.MateriaManifest, error)
	AddSource(Source) error
}
