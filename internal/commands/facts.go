package commands

import (
	"context"
	"fmt"
	"os"

	"charm.land/log/v2"
	"github.com/BurntSushi/toml"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/facts"
)

func RunFacts(ctx context.Context, k *koanf.Koanf, arg string) error {
	m, err := SetupFromCli(ctx, k)
	if err != nil {
		return err
	}
	snap, err := facts.Collect(ctx, m.Collectors...)
	if err != nil {
		return fmt.Errorf("unable to get facts: %w", err)
	}

	defer func() {
		if err := m.Close(); err != nil {
			log.Warn("error closing materia: %w", err)
		}
	}()
	if arg != "" {
		fact, err := snap.LookupString(arg)
		if err != nil {
			return err
		}
		return toml.NewEncoder(os.Stdout).Encode(fact)
	}
	return toml.NewEncoder(os.Stdout).Encode(snap.Snapshot())
}
