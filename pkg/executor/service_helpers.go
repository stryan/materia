package executor

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/services"
)

type ServiceManager interface {
	ApplyService(context.Context, string, services.ServiceAction, int) error
	GetService(context.Context, string) (*services.Service, error)
	RunOneshotCommand(context.Context, int, string, []string) error
	WaitUntilState(context.Context, string, string, int) error
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

	return sm.ApplyService(ctx, res.Service(), cmd, timeout)
}
