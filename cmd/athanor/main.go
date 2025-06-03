package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"git.saintnet.tech/stryan/materia/internal/athanor"
	"git.saintnet.tech/stryan/materia/internal/containers"
	"git.saintnet.tech/stryan/materia/internal/materia"
	"git.saintnet.tech/stryan/materia/internal/repository"
	"git.saintnet.tech/stryan/materia/internal/services"
	"github.com/charmbracelet/log"
	"github.com/urfave/cli/v3"
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
	ctx := context.Background()
	mc, err := materia.NewConfig("")
	if err != nil {
		log.Fatal(err)
	}
	var materiaConfigFile string
	ac, err := athanor.NewConfig("")
	if err != nil {
		log.Fatal(err)
	}
	var athanorConfigFile string

	app := &cli.Command{
		Name:  "athanor",
		Usage: "Backup quadlet volumes",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "materia config",
				Usage:       "Specifed materia config file",
				Required:    false,
				Destination: &materiaConfigFile,
				Aliases:     []string{"m"},
				Action: func(ctx context.Context, cCtx *cli.Command, v string) error {
					if v == "" {
						return errors.New("config file passed wihout value")
					}
					if _, err := os.Stat(v); err != nil && os.IsNotExist(err) {
						return errors.New("config file not found")
					} else if err != nil {
						return err
					}
					mc, err = materia.NewConfig(v)
					return err
				},
			},
			&cli.StringFlag{
				Name:        "athanor config",
				Usage:       "Specifed athanor config file",
				Required:    false,
				Destination: &athanorConfigFile,
				Aliases:     []string{"m"},
				Action: func(ctx context.Context, cCtx *cli.Command, v string) error {
					if v == "" {
						return errors.New("config file passed wihout value")
					}
					if _, err := os.Stat(v); err != nil && os.IsNotExist(err) {
						return errors.New("config file not found")
					} else if err != nil {
						return err
					}
					ac, err = athanor.NewConfig(v)
					return err
				},
			},
		},

		Commands: []*cli.Command{
			{
				Name:  "backup",
				Usage: "Backup all materia managed volumes",
				Action: func(ctx context.Context, cCtx *cli.Command) error {
					repo := &repository.HostComponentRepository{DataPrefix: filepath.Join(mc.MateriaDir, "materia", "components"), QuadletPrefix: mc.QuadletDir}
					compNames, err := repo.ListComponentNames()
					if err != nil {
						return fmt.Errorf("error listing quadlets: %w", err)
					}
					pm := &containers.PodmanManager{}
					defer pm.Close()
					sm, err := services.NewServices(ctx, &services.ServicesConfig{
						Timeout: 30,
					})
					if err != nil {
						return err
					}
					defer sm.Close()
					a, err := athanor.NewAthanor(ac, repo, pm, sm)
					if err != nil {
						return err
					}

					targets := []*athanor.ComponentTarget{}
					for _, name := range compNames {
						c, err := repo.GetComponent(name)
						if err != nil {
							return err
						}
						newTarget, err := a.GenerateTarget(ctx, c)
						if err != nil {
							return err
						}
						targets = append(targets, newTarget)
					}
					for _, t := range targets {
						err := a.BackupTarget(ctx, t)
						if err != nil {
							return err
						}
					}
					return nil
				},
			},
			{
				Name:  "version",
				Usage: "show version",
				Action: func(ctx context.Context, _ *cli.Command) error {
					fmt.Printf("athanor version git-%v\n", Commit)
					return nil
				},
			},
		},
	}

	if err := app.Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
