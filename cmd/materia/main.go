package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"charm.land/log/v2"
	"github.com/urfave/cli/v3"
	"primamateria.systems/materia/internal/commands"
	"primamateria.systems/materia/internal/config"
	"primamateria.systems/materia/internal/rpc"
	"primamateria.systems/materia/internal/server"
)

var Version string

func main() {
	cliflags := make(map[string]any)
	ctx := context.Background()
	var configFile string

	app := &cli.Command{
		Name:                  "materia",
		Usage:                 "Manage quadlet files and resources",
		EnableShellCompletion: true,
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
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			if _, err := os.Stat("/etc/materia/config.toml"); err == nil {
				configFile = "/etc/materia/config.toml"
			}
			return ctx, nil
		},
		Commands: []*cli.Command{
			{
				Name:  "config",
				Usage: "Dump active config",
				Action: func(ctx context.Context, cCtx *cli.Command) error {
					k, err := config.LoadConfigs(ctx, configFile, map[string]any{})
					if err != nil {
						return err
					}
					return commands.RunDumpConfig(ctx, k)
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
					k, err := config.LoadConfigs(ctx, configFile, cliflags)
					if err != nil {
						return err
					}
					return commands.RunFacts(ctx, k, host, arg)
				},
			},
			{
				Name:  "plan",
				Usage: "Show application plan",
				Flags: []cli.Flag{
					&cli.BoolFlag{
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
					k, err := config.LoadConfigs(ctx, configFile, cliflags)
					if err != nil {
						return err
					}
					return commands.RunPlan(ctx, k, quiet, format)
				},
			},
			{
				Name:  "update",
				Usage: "Plan and execute update",
				Flags: []cli.Flag{
					&cli.BoolFlag{
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
					k, err := config.LoadConfigs(ctx, configFile, cliflags)
					if err != nil {
						return err
					}
					return commands.RunUpdate(ctx, k, quiet)
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

					k, err := config.LoadConfigs(ctx, configFile, cliflags)
					if err != nil {
						return err
					}
					return commands.RunRemove(ctx, k, comp)
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
					k, err := config.LoadConfigs(ctx, configFile, cliflags)
					if err != nil {
						return err
					}
					return commands.RunValidate(ctx, k, commands.ValidationSetup{Component: comp, Roles: roles}, cCtx.Bool("verbose"))
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
					remove := cCtx.Bool("remove")
					k, err := config.LoadConfigs(ctx, configFile, map[string]any{})
					if err != nil {
						return err
					}
					return commands.RunDoctor(ctx, k, remove)
				},
			},
			{
				Name:  "server",
				Usage: "start materia in server mode",
				Action: func(ctx context.Context, cCtx *cli.Command) error {
					k, err := config.LoadConfigs(ctx, configFile, cliflags)
					if err != nil {
						return err
					}
					return server.RunServer(ctx, k, Version)
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
							cfg := rpc.AgentConfig{Socket: cCtx.String("socket")}
							agent, err := rpc.NewAgent(cfg)
							if err != nil {
								return err
							}
							return agent.Facts(ctx)
						},
					},
					{
						Name:  "sync",
						Usage: "Sync local repo",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "revision",
								Usage:    "Revision to sync to",
								Required: false,
								Aliases:  []string{"r"},
							},
						},

						Action: func(ctx context.Context, cCtx *cli.Command) error {
							cfg := rpc.AgentConfig{Socket: cCtx.String("socket")}
							agent, err := rpc.NewAgent(cfg)
							if err != nil {
								return err
							}
							var rev *string
							if cCtx.String("revision") != "" {
								revarg := cCtx.String("revision")
								rev = &revarg
							}

							return agent.Sync(ctx, rev)
						},
					},
					{
						Name:  "plan",
						Usage: "Generate a plan",
						Action: func(ctx context.Context, cCtx *cli.Command) error {
							cfg := rpc.AgentConfig{Socket: cCtx.String("socket")}
							agent, err := rpc.NewAgent(cfg)
							if err != nil {
								return err
							}
							return agent.Plan(ctx)
						},
					},
					{
						Name:  "update",
						Usage: "Run update",
						Action: func(ctx context.Context, cCtx *cli.Command) error {
							cfg := rpc.AgentConfig{Socket: cCtx.String("socket")}
							agent, err := rpc.NewAgent(cfg)
							if err != nil {
								return err
							}
							return agent.Update(ctx)
						},
					},
				},
			},
			{
				Name:  "clean",
				Usage: "remove all related file paths",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Usage:   "Don't try to remove components gracefully before cleaning",
					},
				},
				Action: func(ctx context.Context, cCtx *cli.Command) error {
					force := cCtx.Bool("force")

					k, err := config.LoadConfigs(ctx, configFile, cliflags)
					if err != nil {
						return err
					}
					return commands.RunClean(ctx, k, force)
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
