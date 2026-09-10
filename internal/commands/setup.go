package commands

import (
	"context"
	"fmt"

	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/materia"
)

func SetupFromCli(ctx context.Context, k *koanf.Koanf) (*materia.Materia, error) {
	c, err := materia.NewConfig(k)
	if err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}
	err = c.Validate()
	if err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}
	src, err := BuildSource(k)
	if err != nil {
		return nil, err
	}
	return materia.NewFromKoanf(ctx, k, src)
}
