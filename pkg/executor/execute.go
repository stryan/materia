package executor

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/plan"
)

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
	serviceActions := []actions.Action{}
	steps := 0
	// Execute actions
	for _, v := range plan.Steps() {
		err := e.executeAction(ctx, v)
		if err != nil {
			return steps, err
		}

		if v.Todo.IsServiceAction() && v.Target.Kind != components.ResourceTypeHost {
			serviceActions = append(serviceActions, v)
		}

		steps++
	}

	// verify services
	servicesResultMap := make(map[string]string)
	for _, v := range serviceActions {
		serv, err := e.host.GetService(ctx, v.Target.Service())
		if err != nil {
			return steps, err
		}
		switch v.Todo {
		case actions.ActionRestart, actions.ActionStart, actions.ActionReload:
			switch serv.State {
			case "activating", "reloading":
				servicesResultMap[serv.Name] = "active"
			case "failed":
				log.Warn("service failed to start/restart/reload", "service", serv.Name, "state", serv.State)
			default:
			}
		case actions.ActionStop:
			if serv.State == "deactivating" {
				servicesResultMap[serv.Name] = "inactive"
			} else if serv.State != "inactive" {
				log.Warn("service failed to stop", "service", serv.Name, "state", serv.State)
			}
		case actions.ActionEnable, actions.ActionDisable:
		default:
			return steps, errors.New("unknown service action state")
		}
	}
	var servWG sync.WaitGroup
	for serv, state := range servicesResultMap {
		servWG.Add(1)
		go func(serv, state string) {
			defer servWG.Done()
			err := e.host.WaitUntilState(ctx, serv, state, e.defaultTimeout)
			if err != nil {
				log.Warn(err)
			}
		}(serv, state)

	}
	servWG.Wait()
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
