package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/urfave/cli/v3"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/materia"
	"primamateria.systems/materia/pkg/hostman"
)

var Version string

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
					cliflags["nosync"] = true
					return nil
				},
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "config",
				Usage: "Dump active config",
				Action: func(ctx context.Context, cCtx *cli.Command) error {
					k, err := LoadConfigs(ctx, configFile, map[string]any{})
					if err != nil {
						return err
					}
					c, err := materia.NewConfig(k)
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
						fact, err := m.Host.Lookup(arg)
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
					&cli.StringFlag{
						Name:    "format",
						Aliases: []string{"f"},
						Usage:   "Control output format. Supports text,json",
					},
				},
				Action: func(ctx context.Context, cCtx *cli.Command) error {
					quiet := false
					format := "text"
					if cCtx.IsSet("quiet") {
						cliflags["quiet"] = cCtx.Bool("quiet")
						quiet = cCtx.Bool("quiet")
					}
					if cCtx.IsSet("resource-only") {
						cliflags["onlyresource"] = cCtx.Bool("resource-only")
					}
					if cCtx.IsSet("format") {
						format = cCtx.String("format")
					}
					m, err := setup(ctx, configFile, cliflags)
					if err != nil {
						return err
					}
					plan, err := m.Plan(ctx)
					if err != nil {
						return fmt.Errorf("error planning actions: %w", err)
					}
					if !quiet {
						switch format {
						case "text":
							if plan.Empty() {
								fmt.Println("No changes made")
								return nil
							}
							fmt.Println(plan.Pretty())
						case "json":
							jsonPlan, err := plan.ToJson()
							if err != nil {
								return fmt.Errorf("error converting to json: %w", err)
							}
							fmt.Printf("%s", string(jsonPlan))
						default:
							return fmt.Errorf("unsupported output format")
						}
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
				Usage: "Remove a non-corrupted component",
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
						if errors.Is(err, components.ErrCorruptComponent) {
							return cli.Exit("Component is corrupted, try `materia doctor` instead", 1)
						}
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
					k, err := LoadConfigs(ctx, configFile, map[string]any{})
					if err != nil {
						return err
					}
					c, err := materia.NewConfig(k)
					if err != nil {
						return err
					}
					hm, err := hostman.NewHostManager(c)
					if err != nil {
						return err
					}
					corrupted, err := hm.ValidateComponents()
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
						err := hm.PurgeComponentByName(v)
						if err != nil {
							return err
						}
					}
					return nil
				},
			},
			{
				Name:  "server",
				Usage: "start materia in server mode",
				Action: func(_ context.Context, cCtx *cli.Command) error {
					k, err := LoadConfigs(ctx, configFile, cliflags)
					if err != nil {
						return err
					}
					return RunServer(ctx, k)
				},
			},
			{
				Name:  "agent",
				Usage: "send commands to running materia server",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "socket",
						Usage:    "Manually specify materia socket",
						Required: false,
						Aliases:  []string{"s"},
						Sources:  cli.EnvVars("MATERIA_AGENT__SOCKET"),
					},
				},
				Commands: []*cli.Command{
					{
						Name:  "facts",
						Usage: "Request facts",
						Action: func(ctx context.Context, cCtx *cli.Command) error {
							socketPath, err := defaultSocket()
							if err != nil {
								return err
							}
							if cCtx.String("socket") != "" {
								socketPath = cCtx.String("socket")
							}
							return factsCommand(ctx, socketPath)
						},
					},
					{
						Name:  "sync",
						Usage: "Sync local repo",
						Action: func(ctx context.Context, cCtx *cli.Command) error {
							socketPath, err := defaultSocket()
							if err != nil {
								return err
							}
							if cCtx.String("socket") != "" {
								socketPath = cCtx.String("socket")
							}
							return syncCommand(ctx, socketPath)
						},
					},
					{
						Name:  "plan",
						Usage: "Generate a plan",
						Action: func(ctx context.Context, cCtx *cli.Command) error {
							socketPath, err := defaultSocket()
							if err != nil {
								return err
							}
							if cCtx.String("socket") != "" {
								socketPath = cCtx.String("socket")
							}
							return planCommand(ctx, socketPath)
						},
					},
					{
						Name:  "update",
						Usage: "Run update",
						Action: func(ctx context.Context, cCtx *cli.Command) error {
							socketPath, err := defaultSocket()
							if err != nil {
								return err
							}
							if cCtx.String("socket") != "" {
								socketPath = cCtx.String("socket")
							}
							return updateCommand(ctx, socketPath)
						},
					},
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
					fmt.Printf("materia version %v\n", Version)
					return nil
				},
			},
		},
	}

	if err := app.Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
