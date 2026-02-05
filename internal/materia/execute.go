package materia

import (
	"context"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/pkg/plan"
)

func (m *Materia) Execute(ctx context.Context, aplan *plan.Plan) (int, error) {
	defer func() {
		if !m.Executor.CleanupComponents {
			return
		}
		m.validatePostExecute(ctx)
	}()
	return m.Executor.Execute(ctx, aplan)
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
