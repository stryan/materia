package main

import (
	"context"
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

func main() {
	ctx := context.Background()
	mc, err := materia.NewConfig()
	if err != nil {
		log.Fatal(err)
	}
	ac, err := athanor.NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	app := &cli.App{
		Name:  "athanor",
		Usage: "Backup quadlet volumes",
		Commands: []*cli.Command{
			{
				Name:  "backup",
				Usage: "Backup all materia managed volumes",
				Action: func(cCtx *cli.Context) error {
					repo := &repository.HostComponentRepository{DataPrefix: filepath.Join(mc.MateriaDir, "materia", "components"), QuadletPrefix: mc.QuadletDir}

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
					comps, err := repo.List(ctx)
					if err != nil {
						return fmt.Errorf("error listing quadlets: %w", err)
					}

					targets := []*athanor.ComponentTarget{}
					for _, c := range comps {
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
				Action: func(_ *cli.Context) error {
					fmt.Printf("athanor version git-%v\n", Commit)
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
