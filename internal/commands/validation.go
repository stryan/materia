package commands

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/facts"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/materia"
)

type ValidationSetup struct {
	Hostname  string
	Component string
	Roles     []string
}

func ValidateComponentCmd(ctx context.Context, k *koanf.Koanf, cfg ValidationSetup, verbose bool) error {
	m, err := SetupFromCli(ctx, k)
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Close(); err != nil {
			log.Warnf("error closing materia: %v", err)
		}
	}()
	snap, err := facts.Collect(ctx, &ValidationCollector{cfg, m.Manifest})
	if err != nil {
		return err
	}

	plan, err := m.Plan(ctx, snap)
	if err != nil {
		return err
	}
	if verbose {
		fmt.Println(plan.Pretty())
	}
	fmt.Println("OK")
	return nil
}

// Returns a predetermined MateriaFacts instead of fetching them
type ValidationCollector struct {
	cfg  ValidationSetup
	mani *manifests.MateriaManifest
}

func (c *ValidationCollector) Namespace() string {
	return "materia"
}

func (c *ValidationCollector) Collect(ctx context.Context, accum *facts.Facts) (any, error) {
	res := materia.MateriaFacts{
		Hostname:            c.cfg.Hostname,
		InstalledComponents: []string{},
	}
	res.Roles = c.cfg.Roles
	assigned, err := materia.GetAssignedComponents(c.mani, c.cfg.Hostname, c.cfg.Roles)
	if err != nil {
		return nil, err
	}
	if c.cfg.Component != "" {
		assigned = []string{c.cfg.Component}
	}
	res.AssignedComponents = assigned

	if res.AssignedComponents == nil {
		res.AssignedComponents = []string{}
	}

	if res.InstalledComponents == nil {
		res.InstalledComponents = []string{}
	}
	return res, nil
}
