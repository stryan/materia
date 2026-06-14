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
	AddSource(source.Source, *source.SyncOpts, *source.SyncReport, bool) error
	Sync(context.Context, *source.SyncOpts) error
	Rollback(context.Context) error
}
