package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"git.saintnet.tech/stryan/materia/internal/materia"
	"github.com/charmbracelet/log"
	"github.com/urfave/cli/v2"
)

var Commit = func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}

	return ""
}()

func setup(ctx context.Context) (*materia.Materia, *materia.Config, error) {
	// Configure
	c, err := materia.NewConfig()
	if err != nil {
		log.Fatal(err)
	}
	err = c.Validate()
	if err != nil {
		log.Fatal(err)
	}
	if c.Debug {
		log.Default().SetLevel(log.DebugLevel)
		log.Default().SetReportCaller(true)
	}
	sm, err := materia.NewServices(ctx, c)
	if err != nil {
		log.Fatal(err)
	}
	cm, err := materia.NewPodmanManager(c)
	if err != nil {
		log.Fatal(err)
	}
	m, err := materia.NewMateria(ctx, c, sm, cm)
	if err != nil {
		log.Fatal(err)
	}
	return m, c, nil
}

func main() {
	ctx := context.Background()
	app := &cli.App{
		Name:  "materia",
		Usage: "Manage quadlet files and resources",
		Commands: []*cli.Command{
			{
				Name:    "facts",
				Aliases: []string{"-f"},
				Action: func(cCtx *cli.Context) error {
					m, c, err := setup(ctx)
					if err != nil {
						return err
					}
					man, facts, err := m.Facts(ctx, c)
					if err != nil {
						return err
					}
					log.Info(man)
					log.Info(facts)
					return nil
				},
			},
			{
				Name:    "plan",
				Aliases: []string{"-p"},
				Usage:   "Show application plan",
				Action: func(cCtx *cli.Context) error {
					m, c, err := setup(ctx)
					if err != nil {
						return err
					}
					manifest, facts, err := m.Facts(ctx, c)
					if err != nil {
						return fmt.Errorf("error generating facts: %w", err)
					}
					err = m.Prepare(ctx, manifest)
					if err != nil {
						return fmt.Errorf("error preparing system: %w", err)
					}
					plan, err := m.Plan(ctx, manifest, facts)
					if err != nil {
						return fmt.Errorf("error planning actions: %w", err)
					}
					for _, p := range plan {
						fmt.Println(p.Pretty())
					}
					return nil
				},
			},
			{
				Name:    "update",
				Aliases: []string{"-u"},
				Usage:   "Plan and execute update",
				Action: func(cCtx *cli.Context) error {
					m, c, err := setup(ctx)
					if err != nil {
						return err
					}
					manifest, facts, err := m.Facts(ctx, c)
					if err != nil {
						return err
					}
					err = m.Prepare(ctx, manifest)
					if err != nil {
						return err
					}
					plan, err := m.Plan(ctx, manifest, facts)
					if err != nil {
						return err
					}
					err = m.Execute(ctx, facts, plan)
					if err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:    "clean",
				Aliases: []string{},
				Usage:   "remove all related file paths",
				Action: func(_ *cli.Context) error {
					m, _, err := setup(ctx)
					if err != nil {
						return err
					}
					return m.Clean(ctx)
				},
			},
			{
				Name:  "version",
				Usage: "show version",
				Action: func(_ *cli.Context) error {
					fmt.Printf("materia version git-%v", Commit)
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
