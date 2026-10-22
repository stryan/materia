package commands

import (
	"context"
	"fmt"

	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/materia"
)

func ConfigCmd(ctx context.Context, k *koanf.Koanf) error {
	c, err := materia.NewConfig(k)
	if err != nil {
		return err
	}
	fmt.Println(c)
	return nil
}
