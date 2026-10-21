package commands

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/facts"
	"primamateria.systems/materia/pkg/materia"
	"primamateria.systems/materia/pkg/plan"
)

// shared between cli and server
func RunPlan(ctx context.Context, m *materia.Materia, facts *facts.Facts) (*plan.Plan, error) {
	var err error
	if facts == nil {
		facts, err = m.Facts(ctx)
		if err != nil {
			return nil, err
		}
	}
	plan, err := m.Plan(ctx, facts)
	if err != nil {
		return nil, fmt.Errorf("error planning actions: %w", err)
	}
	return plan, nil
}

func PlanCmd(ctx context.Context, k *koanf.Koanf, quiet bool, format string) error {
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
	plan, err := RunPlan(ctx, m, facts)
	if err != nil {
		return fmt.Errorf("error planning actions: %w", err)
	}
	if !quiet {
		switch format {
		case "text":
			if plan.Empty() {
				fmt.Println("No changes made")
				return nil
			}
			fmt.Println(plan.Pretty())
		case "json":
			jsonPlan, err := plan.ToJson()
			if err != nil {
				return fmt.Errorf("error converting to json: %w", err)
			}
			fmt.Printf("%s", string(jsonPlan))
		default:
			return fmt.Errorf("unsupported output format")
		}
	}
	err = m.SavePlan(ctx, plan)
	if err != nil {
		return fmt.Errorf("error writing plan: %w", err)
	}

	return nil
}
