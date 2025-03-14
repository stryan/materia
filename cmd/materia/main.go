package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"git.saintnet.tech/stryan/materia/internal/containers"
	"git.saintnet.tech/stryan/materia/internal/materia"
	"git.saintnet.tech/stryan/materia/internal/services"
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

func setup(ctx context.Context, c *materia.Config) (*materia.Materia, error) {
	// Configure

	err := c.Validate()
	if err != nil {
		log.Fatal(err)
	}
	if c.Debug {
		log.Default().SetLevel(log.DebugLevel)
		log.Default().SetReportCaller(true)
	}
	sm, err := services.NewServices(ctx, &services.ServicesConfig{
		Timeout: c.Timeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	cm, err := containers.NewPodmanManager()
	if err != nil {
		log.Fatal(err)
	}
	m, err := materia.NewMateria(ctx, c, sm, cm)
	if err != nil {
		log.Fatal(err)
	}
	return m, nil
}

func main() {
	ctx := context.Background()
	c, err := materia.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	app := &cli.App{
		Name:  "materia",
		Usage: "Manage quadlet files and resources",
		Commands: []*cli.Command{
			{
				Name:  "facts",
				Usage: "Display host facts",
				Action: func(cCtx *cli.Context) error {
					m, err := setup(ctx, c)
					if err != nil {
						return err
					}
					fmt.Println(m.Facts.Pretty())
					return nil
				},
			},
			{
				Name:  "plan",
				Usage: "Show application plan",
				Action: func(cCtx *cli.Context) error {
					m, err := setup(ctx, c)
					if err != nil {
						return err
					}
					plan, err := m.Plan(ctx)
					if err != nil {
						return fmt.Errorf("error planning actions: %w", err)
					}
					fmt.Println(plan.Pretty())
					return nil
				},
			},
			{
				Name:  "update",
				Usage: "Plan and execute update",
				Action: func(cCtx *cli.Context) error {
					m, err := setup(ctx, c)
					if err != nil {
						return err
					}
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
				Name:  "remove",
				Usage: "Remove a component",
				Action: func(cCtx *cli.Context) error {
					comp := cCtx.Args().First()
					if comp == "" {
						return cli.Exit("specify a component to remove", 1)
					}

					m, err := setup(ctx, c)
					if err != nil {
						return err
					}
					err = m.CleanComponent(ctx, comp)
					if err != nil {
						return cli.Exit(fmt.Sprintf("error removing component: %v", err), 1)
					}
					fmt.Printf("component %v removed succesfully", comp)
					return nil
				},
			},
			{
				Name:  "validate",
				Usage: "Validate a component/repo for a given host/role",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "component",
						Aliases: []string{"c"},
						Usage:   "component to validate",
					},
					&cli.StringFlag{
						Name:    "hostname",
						Aliases: []string{"n"},
						Usage:   "hostname to use for facts generation",
					},
					&cli.StringFlag{
						Name:    "source",
						Aliases: []string{"s"},
						Usage:   "Repo source directory",
					},
					&cli.StringSliceFlag{
						Name:    "roles",
						Aliases: []string{"r"},
						Usage:   "roles to use for facts generation",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Usage:   "show full plan for each tested component",
					},
				},
				Action: func(cCtx *cli.Context) error {
					comp := cCtx.String("component")
					hostname := cCtx.String("hostname")
					roles := cCtx.StringSlice("roles")
					source := cCtx.String("source")
					if hostname == "" && roles == nil {
						return errors.New("validate needs at least one of hostname or roles specified")
					}

					if source == "" {
						source = "./"
					}
					c.SourceURL = fmt.Sprintf("file://%v", source)
					m, err := setup(ctx, c)
					if err != nil {
						return err
					}
					plan, err := m.ValidateComponent(ctx, comp, hostname, roles)
					if err != nil {
						return err
					}
					if cCtx.Bool("verbose") {
						fmt.Println(plan.Pretty())
					} else {
						fmt.Println("OK")
					}

					return nil
				},
			},
			{
				Name:  "clean",
				Usage: "remove all related file paths",
				Action: func(_ *cli.Context) error {
					m, err := setup(ctx, c)
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
