package materia

import (
	"context"

	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/services"
)

type ServiceManager interface {
	ApplyService(context.Context, string, services.ServiceAction, int) error
	GetService(context.Context, string) (*services.Service, error)
	RunOneshotCommand(context.Context, int, string, []string) error
	WaitUntilState(context.Context, string, string, int) error
}

type HostManager interface {
	containers.ContainerManager
	components.ComponentReader
	components.ComponentWriter
	FactsProvider
	ServiceManager
	ListInstalledComponents() ([]string, error)
	InstallScript(context.Context, string, []byte) error
	RemoveScript(context.Context, string) error
	InstallUnit(context.Context, string, []byte) error
	RemoveUnit(context.Context, string) error
}
