package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/containers"
	"git.saintnet.tech/stryan/materia/internal/materia"
	"git.saintnet.tech/stryan/materia/internal/repository"
	"github.com/charmbracelet/log"
	"github.com/containers/podman/v5/pkg/systemd/parser"
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
	c, err := materia.NewConfig()
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
					repo := &repository.HostComponentRepository{DataPrefix: filepath.Join(c.MateriaDir, "materia", "components"), QuadletPrefix: c.QuadletDir}
					comps, err := repo.List(ctx)
					if err != nil {
						return fmt.Errorf("error listing quadlets: %w", err)
					}
					compToContainerMap := make(map[string]string)
					pm := containers.PodmanManager{}
					defer pm.Close()
					outputLocation := "/tmp/"
					containerFiles := []string{}
					for _, c := range comps {
						resources, err := repo.ListResources(ctx, filepath.Base(c))
						if err != nil {
							return fmt.Errorf("error listing resources: %w", err)
						}
						for _, r := range resources {
							if strings.HasSuffix(r, ".container") {
								containerFiles = append(containerFiles, r)
								compToContainerMap[r] = filepath.Base(c)
							}
						}
					}
					for _, c := range containerFiles {
						log.Infof("checking container %v", c)
						// TODO Parse file for Volume= instances
						unitfile, err := parser.ParseUnitFile(c)
						if err != nil {
							return err
						}
						volumes := unitfile.LookupAll("Container", "Volume")
						for _, v := range volumes {
							vs := strings.Split(v, ":")
							if len(vs) < 2 {
								return fmt.Errorf("volume %v is in invalid format", v)
							}
							volFile := vs[0]
							if strings.HasSuffix(volFile, ".volume") {
								// dump backup
								log.Infof("dumping volume %v", volFile)
								component := compToContainerMap[c]
								path, err := repo.Get(ctx, component, volFile)
								if err != nil {
									return err
								}
								volumeUnitFile, err := parser.ParseUnitFile(path)
								if err != nil {
									return err
								}
								volumeName, found := volumeUnitFile.Lookup("Container", "VolumeName")
								if !found {
									volumeName = fmt.Sprintf("systemd-%v", strings.TrimSuffix(volFile, ".volume"))
								}
								err = pm.DumpVolume(ctx, volumeName, outputLocation, true)
								if err != nil {
									return err
								}
							}
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
