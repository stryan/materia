package commands

import (
	"context"
	"fmt"
	"log"

	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/materia"
)

func RunDumpConfig(ctx context.Context, k *koanf.Koanf) error {
	c, err := materia.NewConfig(k)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(c)
	return nil
}
