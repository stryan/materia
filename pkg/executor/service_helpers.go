package executor

import (
	"context"
	"errors"
	"fmt"

	"charm.land/log/v2"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/services"
)

type ErrServiceUnhealthy struct {
	name string
	err  error
}

func (e *ErrServiceUnhealthy) Error() string {
	return fmt.Sprintf("service %v unhealthy: %v", e.name, e.err)
}

type ServiceManager interface {
	ApplyService(context.Context, string, services.ServiceAction, int) error
	GetService(context.Context, string) (*services.Service, error)
	RunOneshotCommand(context.Context, int, string, []string) error
	WaitUntilState(context.Context, string, services.ServiceState, int) error
}

func getServiceType(a actions.Action) (services.ServiceAction, error) {
	switch a.Todo {
	case actions.ActionDisable:
		return services.ServiceDisable, nil
	case actions.ActionEnable:
		return services.ServiceEnable, nil
	case actions.ActionReload:
		if a.Target.Kind == components.ResourceTypeHost {
			return services.ServiceReloadUnits, nil
		}
		return services.ServiceReloadService, nil
	case actions.ActionRestart:
		return services.ServiceRestart, nil
	case actions.ActionStart:
		return services.ServiceStart, nil
	case actions.ActionStop:
		return services.ServiceStop, nil
	default:
		panic(fmt.Sprintf("unexpected actions.ActionType: %#v", a))
	}
}

func modifyService(ctx context.Context, sm ServiceManager, command actions.Action, timeout int) error {
	if err := command.Validate(); err != nil {
		return err
	}
	res := command.Target
	if !res.IsQuadlet() && (res.Kind != components.ResourceTypeService && res.Kind != components.ResourceTypeHost) {
		return fmt.Errorf("tried to modify resource %v as a service", res)
	}
	if command.Metadata != nil && command.Metadata.ServiceTimeout != nil {
		timeout = *command.Metadata.ServiceTimeout
	}
	if err := res.Validate(); err != nil {
		return fmt.Errorf("invalid resource when modifying service: %w", err)
	}
	cmd, err := getServiceType(command)
	if err != nil {
		return err
	}
	log.Debugf("%v service %v", cmd, res.Service())

	err = sm.ApplyService(ctx, res.Service(), cmd, timeout)
	if err != nil {
		if errors.Is(err, services.ErrStateChangeFailed) {
			return &ErrServiceUnhealthy{res.Service(), err}
		}
		return err
	}
	return nil
}

func waitService(ctx context.Context, sm ServiceManager, command actions.Action, timeout int) error {
	if err := command.Validate(); err != nil {
		return err
	}
	if command.Metadata == nil {
		return nil
	}
	if command.Metadata.ServiceUntilState == nil {
		return nil
	}
	res := command.Target
	if !res.IsQuadlet() && (res.Kind != components.ResourceTypeService && res.Kind != components.ResourceTypeHost) {
		return fmt.Errorf("tried to wait on resource %v as a service", res)
	}
	if command.Metadata.ServiceTimeout != nil {
		timeout = *command.Metadata.ServiceTimeout
	}
	endState := services.NewServiceState(*command.Metadata.ServiceUntilState)

	if err := res.Validate(); err != nil {
		return fmt.Errorf("invalid resource when modifying service: %w", err)
	}
	log.Debugf("service %v waiting for %v", res.Service(), endState)

	err := sm.WaitUntilState(ctx, res.Service(), endState, timeout)
	if err != nil {

		if errors.Is(err, services.ErrStateChangeFailed) {
			return &ErrServiceUnhealthy{res.Service(), err}
		}
		return err
	}
	return nil
}
