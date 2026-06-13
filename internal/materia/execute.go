package materia

import (
	"context"
	"errors"
	"fmt"

	"charm.land/log/v2"
	"primamateria.systems/materia/pkg/executor"
	"primamateria.systems/materia/pkg/plan"
)

var ErrNeedRollback = errors.New("need rollback")

type ExecutionReport struct {
	StepsCompleted int
	Rolledback     bool
	Error          error
}

func (m *Materia) Execute(ctx context.Context, aplan *plan.Plan) (ExecutionReport, error) {
	defer func() {
		if m.Executor.CleanupComponents {
			m.validatePostExecute(ctx)
		}
	}()

	if err := m.lock(ctx); err != nil {
		return ExecutionReport{}, fmt.Errorf("unable to get materia dbus lock: %v", err)
	}
	defer m.unlock()
	steps, err := m.Executor.Execute(ctx, aplan)
	if aErr, ok := errors.AsType[*executor.ErrServiceUnhealthy](err); ok {
		if m.Rollback {
			return ExecutionReport{StepsCompleted: steps, Rolledback: true, Error: aErr}, ErrNeedRollback
		}
	}
	if aErr, ok := errors.AsType[*executor.ErrFinalStateUnhealthy](err); ok {
		// services are unhealthy on final check, rollback if enabled
		if m.Rollback {
			return ExecutionReport{StepsCompleted: steps, Rolledback: true, Error: aErr}, ErrNeedRollback
		}
	}
	if err != nil {
		return ExecutionReport{}, err
	}

	return ExecutionReport{steps, false, nil}, nil
}

func (m *Materia) validatePostExecute(ctx context.Context) {
	problems, err := m.ValidateComponents(ctx)
	if err != nil {
		log.Warnf("error cleaning up execution: %v", err)
	}
	for _, v := range problems {
		log.Infof("component %v failed to install, purging", v)
		err := m.Host.PurgeComponentByName(v)
		if err != nil {
			log.Warnf("error purging component: %v", err)
		}
	}
}
