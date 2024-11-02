package main

import (
	"context"
	"os"

	"git.saintnet.tech/stryan/materia/internal/materia"
	"github.com/charmbracelet/log"
	"github.com/urfave/cli/v2"
)

func main() {
	// Configure
	c := materia.NewConfig()

	ctx := context.Background()
	m, err := materia.NewMateria(ctx, c)
	if err != nil {
		log.Fatal(err)
	}
	defer m.Close()
	log.Debug("dump", "materia", m)
	app := &cli.App{
		Name:  "materia",
		Usage: "Manage quadlet files",
		Commands: []*cli.Command{
			{
				Name:    "plan",
				Aliases: []string{"-p"},
				Usage:   "Show application plan",
				Action: func(cCtx *cli.Context) error {
					plan, err := m.Plan(ctx)
					if err != nil {
						return err
					}
					log.Info(plan)
					return nil
				},
			},
			{
				Name:    "update",
				Aliases: []string{"-u"},
				Usage:   "Plan and execute update",
				Action: func(cCtx *cli.Context) error {
					plan, err := m.Plan(ctx)
					if err != nil {
						return err
					}
					err = m.Execute(ctx, plan)
					if err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:    "help",
				Aliases: []string{"-h"},
				Usage:   "show usage",
				Action: func(cCtx *cli.Context) error {
					return nil
				},
			},
			{
				Name:    "clean",
				Aliases: []string{},
				Usage:   "remove all related file paths",
				Action: func(_ *cli.Context) error {
					return m.Clean(ctx)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
