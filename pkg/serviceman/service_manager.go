package serviceman

import (
	"context"

	"primamateria.systems/materia/internal/services"
)

type ServiceManager interface {
	Apply(context.Context, string, services.ServiceAction, int) error
	GetService(context.Context, string) (*services.Service, error)
	RunOneshotCommand(context.Context, int, string, []string) error
	WaitUntilState(context.Context, string, string, int) error
}
