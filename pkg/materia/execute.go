package materia

import (
	"context"
	"errors"
	"fmt"
	"time"

	"charm.land/log/v2"
	"primamateria.systems/materia/pkg/executor"
	"primamateria.systems/materia/pkg/plan"
)

var ErrNeedRollback = errors.New("need rollback")

type ErrNeedRollbackType struct {
	Reason error
}

func (e *ErrNeedRollbackType) Error() string {
	return fmt.Sprintf("need rollback: %v", e.Reason)
}

type ExecutionReport struct {
	Timestamp      time.Time
	StepsCompleted int
	TotalSteps     int
	Error          error
}

// Execute always returns StepsCompleted in its ExecutionReport, even if it errors out
func (m *Materia) Execute(ctx context.Context, aplan *plan.Plan) (ExecutionReport, error) {
	defer func() {
		if m.Executor.CleanupComponents {
			m.validatePostExecute(ctx)
		}
	}()

	if err := m.lock(ctx); err != nil {
		return ExecutionReport{}, fmt.Errorf("unable to get materia dbus lock: %v", err)
	}

	startTime := time.Now()
	defer m.unlock()
	steps, err := m.Executor.Execute(ctx, aplan)
	report := ExecutionReport{
		Timestamp:      startTime,
		StepsCompleted: steps,
		TotalSteps:     aplan.Size(),
	}

	var rollbackErr error
	if aErr, ok := errors.AsType[*executor.ErrServiceUnhealthy](err); ok {
		rollbackErr = &ErrNeedRollbackType{aErr}
	}
	if aErr, ok := errors.AsType[*executor.ErrFinalStateUnhealthy](err); ok {
		rollbackErr = &ErrNeedRollbackType{aErr}
	}
	if m.Rollback && rollbackErr != nil {
		report.Error = rollbackErr
		return report, rollbackErr
	}
	report.Error = err
	return report, err
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
