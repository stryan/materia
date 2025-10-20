package materia

import (
	"context"
	"errors"

	"primamateria.systems/materia/internal/services"
)

type ServiceManager interface {
	Apply(context.Context, string, services.ServiceAction) error
	Get(context.Context, string) (*services.Service, error)
	WaitUntilState(context.Context, string, string) error
}

func getLiveService(ctx context.Context, sm ServiceManager, serviceName string) (*services.Service, error) {
	if sm == nil {
		return nil, errors.New("need service manager")
	}
	if serviceName == "" {
		return nil, errors.New("need service name")
	}
	liveService, err := sm.Get(ctx, serviceName)
	if errors.Is(err, services.ErrServiceNotFound) {
		return &services.Service{
			Name:    serviceName,
			State:   "non-existent",
			Enabled: false,
		}, nil
	}
	return liveService, err
}
