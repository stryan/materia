package executor

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"charm.land/log/v2"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/plan"
	"primamateria.systems/materia/pkg/services"
)

type ErrFinalStateUnhealthy struct {
	Expected services.ServicesSlice
	Actual   services.ServicesSlice
}

func (e *ErrFinalStateUnhealthy) Error() string {
	return fmt.Sprintf("Execute in unhealthy state: Expected %v, Actual%v", e.Expected, e.Actual)
}

type Executor struct {
	ExecutorConfig
	host           Host
	defaultTimeout int
}

func NewExecutor(conf ExecutorConfig, host Host, timeout int) *Executor {
	return &Executor{
		conf,
		host,
		timeout,
	}
}

func (e *Executor) Execute(ctx context.Context, plan *plan.Plan) (int, error) {
	if plan.Empty() {
		return -1, nil
	}
	lastAction := make(map[string]actions.ActionType)
	expectedServices := make(services.ServicesSlice)
	steps := 0
	// Execute actions
	for _, v := range plan.Steps() {
		err := e.executeAction(ctx, v)
		if err != nil {
			return steps, err
		}

		if v.Todo.IsServiceAction() && v.Target.Kind != components.ResourceTypeHost {
			if v.Metadata != nil {
				if v.Metadata.ServiceUntilState != nil {
					// we know exact what state it should be in, don't bother guessing
					expectedServices[v.Target.Service()] = services.NewServiceState(*v.Metadata.ServiceUntilState)
					continue
				}
			}
			lastAction[v.Target.Service()] = v.Todo
		}

		steps++
	}

	// verify services are in their expected end state (i.e. the result of the last service change command)
	for k, v := range lastAction {
		serv, err := e.host.GetService(ctx, k)
		if err != nil {
			if errors.Is(err, services.ErrServiceNotFound) && v == actions.ActionStop {
				// nothing to do if the service is fully gone
				continue
			}
			return steps, fmt.Errorf("unable to get service %v for final check: %w", k, err)
		}
		switch v {
		case actions.ActionRestart, actions.ActionStart, actions.ActionReload:
			expectedServices[serv.Name] = services.StateActive
		case actions.ActionStop:
			expectedServices[serv.Name] = services.StateInactive
		case actions.ActionEnable, actions.ActionDisable:
		default:
			return steps, fmt.Errorf("unknown service action state: %v", v)
		}
	}
	finalServices := make(services.ServicesSlice)
	var servWG sync.WaitGroup
	badState := false
	for serv, state := range expectedServices {
		servWG.Add(1)
		go func(serv string, state services.ServiceState) {
			defer servWG.Done()
			err := e.host.WaitUntilState(ctx, serv, state, e.defaultTimeout) // TODO dynamically adjust timeout
			if err == nil {
				finalServices[serv] = state
				return
			}
			if !errors.Is(err, services.ErrOperationTimedOut) {
				log.Warn("Error waiting for final service %v check: %w", serv, err)
			}
			badState = true
			fserv, err := e.host.GetService(ctx, serv)
			if err != nil {
				if errors.Is(err, services.ErrServiceNotFound) && state == services.StateInactive {
					// nothing to do if the service is fully gone
					return
				}
				finalServices[serv] = services.StateUnknown
				return
			}
			finalServices[serv] = fserv.State
		}(serv, state)
	}
	servWG.Wait()
	if badState {
		return steps, &ErrFinalStateUnhealthy{
			Expected: expectedServices,
			Actual:   finalServices,
		}
	}
	return steps, nil
}

func (e *Executor) executeAction(ctx context.Context, v actions.Action) error {
	handlers, ok := handlerList[v.Target.Kind]
	if !ok {
		return fmt.Errorf("unsupported resource type: %v", v.Target.Kind)
	}

	handler, ok := handlers[v.Todo]
	if !ok {
		return fmt.Errorf("no %v handler registered for action type %v", v.Target.Kind, v.Todo)
	}
	return handler(ctx, e, v)
}
