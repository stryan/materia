package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/urfave/cli/v3"
	"primamateria.systems/materia/internal/materia"
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

func main() {
	cliflags := make(map[string]any)
	ctx := context.Background()

	var configFile string

	app := &cli.Command{
		Name:  "materia",
		Usage: "Manage quadlet files and resources",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Usage:       "Specifed TOML config file",
				Required:    false,
				Destination: &configFile,
				Aliases:     []string{"c"},
				Sources:     cli.EnvVars("MATERIA_CONFIG"),
				Action: func(ctx context.Context, cCtx *cli.Command, v string) error {
					if v == "" {
						return errors.New("config file passed wihout value")
					}
					if _, err := os.Stat(v); err != nil && os.IsNotExist(err) {
						return errors.New("config file not found")
					} else if err != nil {
						return err
					}
					return nil
				},
			},
			&cli.BoolFlag{
				Name:     "nosync",
				Usage:    "Disable syncing for commands that sync",
				Required: false,
				Sources:  cli.EnvVars("MATERIA_NOSYNC"),
				Action: func(ctx context.Context, cm *cli.Command, b bool) error {
					cliflags["source.nosync"] = true
					return nil
				},
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "config",
				Usage: "Dump active config",
				Action: func(ctx context.Context, cCtx *cli.Command) error {
					k := koanf.New(".")
					err := k.Load(env.Provider("MATERIA", ".", func(s string) string {
						return strings.ReplaceAll(strings.ToLower(
							strings.TrimPrefix(s, "MATERIA_")), "_", ".")
					}), nil)
					if err != nil {
						return fmt.Errorf("error loading config from env: %w", err)
					}
					if configFile != "" {
						err = k.Load(file.Provider(configFile), toml.Parser())
						if err != nil {
							return fmt.Errorf("error loading config file: %w", err)
						}
					}
					c, err := materia.NewConfig(k, cliflags)
					if err != nil {
						log.Fatal(err)
					}
					fmt.Println(c)
					return nil
				},
			},
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
				Action: func(ctx context.Context, cCtx *cli.Command) error {
					host := cCtx.Bool("host")
					arg := cCtx.String("fact")
					m, err := setup(ctx, configFile, cliflags)
					if err != nil {
						return err
					}
					if arg != "" {
						fact, err := m.LookupFact(arg)
						if err != nil {
							return err
						}
						fmt.Printf("Fact %v: %v", arg, fact)
						return nil
					}
					fmt.Println(m.GetFacts(host))
					return nil
				},
			},
			{
				Name:  "plan",
				Usage: "Show application plan",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "quiet",
						Aliases: []string{"q"},
						Usage:   "Minimize output",
					},
					&cli.BoolFlag{
						Name:    "resource-only",
						Aliases: []string{"r"},
						Usage:   "Only install resources",
					},
				},
				Action: func(ctx context.Context, cCtx *cli.Command) error {
					quiet := false
					if cCtx.IsSet("quiet") {
						cliflags["quiet"] = cCtx.Bool("quiet")
						quiet = cCtx.Bool("quiet")
					}
					if cCtx.IsSet("resource-only") {
						cliflags["onlyresource"] = cCtx.Bool("resource-only")
					}
					m, err := setup(ctx, configFile, cliflags)
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
					if !quiet {
						fmt.Println(plan.Pretty())
					}
					err = m.SavePlan(plan, "plan.toml")
					if err != nil {
						return fmt.Errorf("error writing plan: %w", err)
					}

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
					&cli.BoolFlag{
						Name:    "resource-only",
						Aliases: []string{"r"},
						Usage:   "Only install resources",
					},
				},
				Action: func(ctx context.Context, cCtx *cli.Command) error {
					quiet := false
					if cCtx.IsSet("quiet") {
						cliflags["quiet"] = cCtx.Bool("quiet")
						quiet = cCtx.Bool("quiet")
					}
					if cCtx.IsSet("resource-only") {
						cliflags["onlyresource"] = cCtx.Bool("resource-only")
					}
					m, err := setup(ctx, configFile, cliflags)
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
					err = m.SavePlan(plan, "lastrun.toml")
					if err != nil {
						return fmt.Errorf("error writing plan: %w", err)
					}
					return nil
				},
			},
			{
				Name:  "remove",
				Usage: "Remove a component",
				Action: func(ctx context.Context, cCtx *cli.Command) error {
					comp := cCtx.Args().First()
					if comp == "" {
						return cli.Exit("specify a component to remove", 1)
					}

					m, err := setup(ctx, configFile, cliflags)
					if err != nil {
						return err
					}
					err = m.CleanComponent(ctx, comp)
					if err != nil {
						return cli.Exit(fmt.Sprintf("error removing component: %v", err), 1)
					}
					fmt.Printf("component %v removed succesfully\n", comp)
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
				Action: func(ctx context.Context, cCtx *cli.Command) error {
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
					cliflags["source.url"] = fmt.Sprintf("file://%v", source)
					if hostname != "" {
						cliflags["hostname"] = hostname
					}
					m, err := setup(ctx, configFile, cliflags)
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
				Action: func(ctx context.Context, cCtx *cli.Command) error {
					// use a fake materia since we can't generate valid facts
					m, err := doctorSetup(ctx, configFile, cliflags)
					if err != nil {
						return err
					}
					corrupted, err := m.ValidateComponents(ctx)
					if err != nil {
						return err
					}
					for _, v := range corrupted {
						fmt.Printf("Corrupted component: %v\n", v)
					}
					if !cCtx.Bool("remove") {
						return nil
					}
					for _, v := range corrupted {
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
				Action: func(_ context.Context, _ *cli.Command) error {
					m, err := setup(ctx, configFile, cliflags)
					if err != nil {
						return err
					}
					return m.Clean(ctx)
				},
			},
			{
				Name:  "version",
				Usage: "show version",
				Action: func(_ context.Context, _ *cli.Command) error {
					fmt.Printf("materia version git-%v\n", Commit)
					return nil
				},
			},
		},
	}

	if err := app.Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
