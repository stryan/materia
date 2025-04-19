package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"git.saintnet.tech/stryan/materia/internal/containers"
	"git.saintnet.tech/stryan/materia/internal/materia"
	"git.saintnet.tech/stryan/materia/internal/repository"
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
	if c.UseStdout {
		log.Default().SetOutput(os.Stdout)
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
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "host",
						Usage: "Return only host facts (i.e. no assigned roles)",
					},
					&cli.StringFlag{
						Name:    "fact",
						Usage:   "Lookup a fact",
						Aliases: []string{"f"},
					},
				},
				Action: func(cCtx *cli.Context) error {
					host := cCtx.Bool("host")
					arg := cCtx.String("fact")
					var facts *materia.Facts
					if host {
						cm, err := containers.NewPodmanManager()
						if err != nil {
							return err
						}
						facts, err = materia.NewFacts(ctx, c, nil, nil, cm)
						if err != nil {
							return err
						}
					} else {
						m, err := setup(ctx, c)
						if err != nil {
							return err
						}
						facts = m.Facts
					}
					if arg != "" {
						fact, err := facts.Lookup(arg)
						if err != nil {
							return err
						}
						fmt.Printf("Fact %v: %v", arg, fact)
						return nil
					}
					fmt.Println(facts.Pretty())
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
					if plan.Empty() {
						fmt.Println("No changes being made")
						return nil
					}
					fmt.Println(plan.Pretty())
					return nil
				},
			},
			{
				Name:  "update",
				Usage: "Plan and execute update",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "quiet",
						Aliases: []string{"q"},
						Usage:   "Minimize output",
					},
				},
				Action: func(cCtx *cli.Context) error {
					quiet := cCtx.Bool("quiet")
					m, err := setup(ctx, c)
					if err != nil {
						return err
					}
					plan, err := m.Plan(ctx)
					if err != nil {
						return err
					}
					if !quiet {
						fmt.Println(plan.Pretty())
					}
					steps, err := m.Execute(ctx, plan)
					if err != nil {
						log.Warnf("%v/%v steps completed", steps, len(plan.Steps()))
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
					if hostname != "" {
						c.Hostname = hostname
					}
					m, err := setup(ctx, c)
					if err != nil {
						return err
					}
					plan, err := m.PlanComponent(ctx, comp, roles)
					if err != nil {
						return err
					}
					if cCtx.Bool("verbose") {
						fmt.Println(plan.Pretty())
					}
					fmt.Println("OK")
					return nil
				},
			},
			{
				Name:  "doctor",
				Usage: "remove corrupted installed components. Dry run by default",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "remove",
						Aliases: []string{"r"},
						Usage:   "Actually remove corrupted components",
					},
				},
				Action: func(cCtx *cli.Context) error {
					// use a fake materia since we can't generate valid facts
					m := &materia.Materia{
						CompRepo: &repository.HostComponentRepository{DataPrefix: filepath.Join(c.MateriaDir, "materia", "components"), QuadletPrefix: c.QuadletDir},
					}
					corruped, err := m.ValidateComponents(ctx)
					if err != nil {
						return err
					}
					for _, v := range corruped {
						fmt.Printf("Corrupted component: %v\n", v)
					}
					if !cCtx.Bool("remove") {
						return nil
					}
					for _, v := range corruped {
						err := m.PurgeComponenet(ctx, v)
						if err != nil {
							return err
						}
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
					fmt.Printf("materia version git-%v\n", Commit)
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
