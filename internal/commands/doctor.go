package commands

import (
	"context"
	"fmt"

	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/hostman"
	"primamateria.systems/materia/pkg/materia"
)

func RunDoctor(ctx context.Context, k *koanf.Koanf, remove bool) error {
	c, err := materia.NewConfig(k)
	if err != nil {
		return err
	}
	hmc := &hostman.HostmanConfig{
		Hostname:         c.Hostname,
		DataDir:          c.MateriaDir,
		QuadletDir:       c.QuadletDir,
		ScriptsDir:       c.ScriptsDir,
		ServicesDir:      c.ServiceDir,
		ServicesConfig:   c.ServicesConfig,
		ContainersConfig: c.ContainersConfig,
		CommandPodman:    c.CommandPodman,
	}
	hm, err := hostman.NewHostManager(ctx, hmc)
	if err != nil {
		return err
	}
	corrupted, err := hm.ValidateComponents()
	if err != nil {
		return err
	}
	for _, v := range corrupted {
		fmt.Printf("Corrupted component: %v\n", v)
	}
	if remove {
		for _, v := range corrupted {
			err := hm.PurgeComponentByName(v)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
