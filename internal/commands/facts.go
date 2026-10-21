package commands

import (
	"context"
	"fmt"
	"os"

	"charm.land/log/v2"
	"github.com/BurntSushi/toml"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/facts"
	"primamateria.systems/materia/pkg/materia"
)

func FactsCmd(ctx context.Context, k *koanf.Koanf, arg string) error {
	m, err := SetupFromCli(ctx, k)
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Close(); err != nil {
			log.Warnf("error closing materia: %v", err)
		}
	}()
	snap, err := RunFacts(ctx, m)
	if err != nil {
		return err
	}
	if arg != "" {
		fact, err := snap.LookupString(arg)
		if err != nil {
			return err
		}
		return toml.NewEncoder(os.Stdout).Encode(fact)
	}
	return toml.NewEncoder(os.Stdout).Encode(snap.Snapshot())
}

func RunFacts(ctx context.Context, m *materia.Materia) (*facts.Facts, error) {
	snap, err := facts.Collect(ctx, m.Collectors...)
	if err != nil {
		return nil, fmt.Errorf("unable to get facts: %w", err)
	}

	return snap, nil
}
