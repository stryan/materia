package materia

import (
	"context"

	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/serviceman"
)

type HostManager interface {
	serviceman.ServiceManager
	containers.ContainerManager
	components.ComponentReader
	components.ComponentWriter
	FactsProvider
	ListInstalledComponents() ([]string, error)
	InstallScript(context.Context, string, []byte) error
	RemoveScript(context.Context, string) error
	InstallUnit(context.Context, string, []byte) error
	RemoveUnit(context.Context, string) error
}
