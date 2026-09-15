package commands

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/knadh/koanf/v2"
)

func RunPlan(ctx context.Context, k *koanf.Koanf, quiet bool, format string) error {
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
	err = m.SavePlan(plan, "plan.toml")
	if err != nil {
		return fmt.Errorf("error writing plan: %w", err)
	}

	return nil
}

type ValidationSetup struct {
	Component string
	Roles     []string
}

func RunValidate(ctx context.Context, k *koanf.Koanf, cfg ValidationSetup, verbose bool) error {
	m, err := SetupFromCli(ctx, k)
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Close(); err != nil {
			log.Warn("error closing materia: %w", err)
		}
	}()

	plan, err := m.PlanComponent(ctx, cfg.Component, cfg.Roles)
	if err != nil {
		return err
	}
	if verbose {
		fmt.Println(plan.Pretty())
	}
	fmt.Println("OK")
	return nil
}
