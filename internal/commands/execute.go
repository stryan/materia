package commands

import (
	"context"
	"errors"
	"fmt"

	"charm.land/log/v2"
	"github.com/knadh/koanf/v2"
	"github.com/urfave/cli/v3"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/materia"
	"primamateria.systems/materia/pkg/notify"
	"primamateria.systems/materia/pkg/plan"
)

func executeAndSave(ctx context.Context, m *materia.Materia, p *plan.Plan) (materia.ExecutionReport, error) {
	rep, err := m.Execute(ctx, p)
	if serr := m.SaveExecution(ctx, p, rep); serr != nil {
		log.Warnf("unable to save execution report: %v", serr)
	}
	return rep, err
}

// RunExecute always returns the final plan executed (which may differ due to rollback), the final plans report, and any errors
// Shared between cli and server
func RunExecute(ctx context.Context, m *materia.Materia, aplan *plan.Plan) (*plan.Plan, materia.ExecutionReport, error) {
	report, err := executeAndSave(ctx, m, aplan)
	if rErr, ok := errors.AsType[*materia.ErrNeedRollbackType](err); ok {
		// Need to run rollback
		err := m.Notifier.Notify(ctx, notify.NotifyRollback, fmt.Sprintf("Rollback initiated; Reason: %v", rErr))
		if err != nil {
			log.Warnf("rolling back but failed to send rollback notification: %v", err)
		}
		err = m.Source.Rollback(ctx)
		if err != nil {
			return aplan, report, fmt.Errorf("rollback failed: %w", err)
		}
		// pass nil facts to force a fresh facts gathering
		rollbackPlan, err := RunPlan(ctx, m, nil)
		if err != nil {
			return aplan, report, fmt.Errorf("rollback plan generation failed: %w", err)
		}
		rollbackReport, err := executeAndSave(ctx, m, rollbackPlan)
		if err != nil {
			return rollbackPlan, rollbackReport, fmt.Errorf("rollback execution failed: %w", err)
		}
		return rollbackPlan, rollbackReport, nil
	}
	if err != nil {
		return aplan, report, err
	}
	return aplan, report, nil
}

func UpdateCmd(ctx context.Context, k *koanf.Koanf, quiet bool) error {
	m, err := SetupFromCli(ctx, k)
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Close(); err != nil {
			log.Warnf("error closing materia: %v", err)
		}
	}()
	facts, err := m.Facts(ctx)
	if err != nil {
		return err
	}
	aplan, err := RunPlan(ctx, m, facts)
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Println(aplan.Pretty())
	}
	// From this point on we log or concatenate errors
	finalPlan, report, exErr := RunExecute(ctx, m, aplan)
	if exErr != nil {
		log.Warnf("Execution: %v/%v steps completed", report.StepsCompleted, report.TotalSteps)
	}
	// may want to compare plan values in the future and not just that its the same pointer
	if !quiet && finalPlan != aplan {
		fmt.Printf("Rollback plan:\n%v", finalPlan.Pretty())
	}
	saveErr := m.SavePlan(ctx, finalPlan)
	return errors.Join(exErr, saveErr)
}

func RemoveCmd(ctx context.Context, k *koanf.Koanf, comp string) error {
	m, err := SetupFromCli(ctx, k)
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Close(); err != nil {
			log.Warn("error closing materia: %w", err)
		}
	}()

	err = m.CleanComponent(ctx, comp)
	if err != nil {
		if errors.Is(err, components.ErrCorruptComponent) {
			return cli.Exit("Component is corrupted, try `materia doctor` instead", 1)
		}
		return cli.Exit(fmt.Sprintf("error removing component: %v", err), 1)
	}
	fmt.Printf("component %v removed succesfully\n", comp)
	return nil
}

func CleanCmd(ctx context.Context, k *koanf.Koanf, force bool) error {
	m, err := SetupFromCli(ctx, k)
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Close(); err != nil {
			log.Warn("error closing materia: %w", err)
		}
	}()

	return m.Clean(ctx, force)
}
