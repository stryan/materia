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
)

func RunUpdate(ctx context.Context, k *koanf.Koanf, quiet bool) error {
	m, err := SetupFromCli(ctx, k)
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Close(); err != nil {
			log.Warn("error closing materia: %w", err)
		}
	}()
	plan, err := m.Plan(ctx)
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Println(plan.Pretty())
	}
	rep, err := m.Execute(ctx, plan)
	if err != nil {
		if rErr, ok := errors.AsType[*materia.ErrNeedRollbackType](err); !ok {
			log.Warnf("%v/%v steps completed", rep.StepsCompleted, len(plan.Steps()))
			return err
		} else {
			err := m.Notifier.Notify(ctx, notify.NotifyRollback, fmt.Sprintf("Rollback initiated; Reason: %v", rErr))
			if err != nil {
				return fmt.Errorf("needed rollback but failed to send rollback notification: %w", err)
			}
			err = m.Source.Rollback(ctx)
			if err != nil {
				return err
			}
			// re-use plan so we save the rolled-back plan outside of this block
			oldPlan := plan
			plan, err = m.Plan(ctx)
			if err != nil {
				return err
			}
			if !quiet {
				fmt.Println(plan.Pretty())
			}
			rep2, err := m.Execute(ctx, plan)
			if err != nil {
				log.Warnf("post-rollback: %v/%v steps completed", rep2.StepsCompleted, len(plan.Steps()))
				return err
			}
			if !quiet {
				fmt.Printf("Original Plan:\n %v\nRollback Plan:\n %v\n", oldPlan.Pretty(), plan.Pretty())
			}
		}
	}
	err = m.SavePlan(plan, "lastrun.toml")
	if err != nil {
		return fmt.Errorf("error writing plan: %w", err)
	}
	return nil
}

func RunRemove(ctx context.Context, k *koanf.Koanf, comp string) error {
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

func RunClean(ctx context.Context, k *koanf.Koanf, force bool) error {
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
