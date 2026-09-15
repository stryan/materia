package commands

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/knadh/koanf/v2"
)

func RunFacts(ctx context.Context, k *koanf.Koanf, host bool, arg string) error {
	m, err := SetupFromCli(ctx, k)
	if err != nil {
		return err
	}

	defer func() {
		if err := m.Close(); err != nil {
			log.Warn("error closing materia: %w", err)
		}
	}()
	if arg != "" {
		fact, err := m.Host.Lookup(arg)
		if err != nil {
			return err
		}
		fmt.Printf("Fact %v: %v", arg, fact)
		return nil
	}
	fmt.Println(m.GetFacts(host))
	return nil
}
