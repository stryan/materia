package materia

import (
	"context"

	"primamateria.systems/materia/internal/services"
)

type ServiceManager interface {
	Apply(context.Context, string, services.ServiceAction, int) error
	Get(context.Context, string) (*services.Service, error)
	WaitUntilState(context.Context, string, string, int) error
}
