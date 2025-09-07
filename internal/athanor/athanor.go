package athanor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/containers/podman/v5/pkg/systemd/parser"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/manifests"
	"primamateria.systems/materia/internal/repository"
	"primamateria.systems/materia/internal/services"
)

type Athanor struct {
	Destination string
	Repo        *repository.HostComponentRepository
	pm          ContainerManager
	sm          Services
}

type ComponentTarget struct {
	Name       string
	Manifest   *manifests.ComponentManifest
	Containers []containers.Container
}

type Config struct {
	OutputDir string
}

func NewConfig(configFile string) (*Config, error) {
	k := koanf.New(".")
	err := k.Load(env.Provider("ATHANOR", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(
			strings.TrimPrefix(s, "ATHANOR_")), "_", ".")
	}), nil)
	if err != nil {
		return nil, err
	}
	if configFile != "" {
		err = k.Load(file.Provider(configFile), toml.Parser())
		if err != nil {
			return nil, err
		}
	}
	var c Config
	c.OutputDir = k.String("outputdir")

	return &c, nil
}

func (c *Config) Validate() error {
	if c.OutputDir == "" {
		return errors.New("need output directory")
	}
	return nil
}

func NewAthanor(conf *Config, repo *repository.HostComponentRepository, pm ContainerManager, sm services.Services) (*Athanor, error) {
	return &Athanor{
		Destination: conf.OutputDir,
		Repo:        repo,
		pm:          pm,
		sm:          sm,
	}, nil
}

func (a *Athanor) GenerateTarget(ctx context.Context, comp *components.Component) (*ComponentTarget, error) {
	newTarget := ComponentTarget{
		Name: comp.Name,
	}
	// returns full paths!
	resources, err := a.Repo.ListResources(comp)
	if err != nil {
		return nil, fmt.Errorf("error listing resources: %w", err)
	}
	for _, r := range resources {
		if r.Kind == components.ResourceTypeContainer {
			container := containers.Container{}
			containerFileData, err := a.Repo.ReadResource(r)
			if err != nil {
				return nil, err
			}
			unitfile := parser.NewUnitFile()
			err = unitfile.Parse(containerFileData)
			if err != nil {
				return nil, fmt.Errorf("error parsing container file: %w", err)
			}
			name, foundName := unitfile.Lookup("Container", "ContainerName")
			if !foundName {
				name = strings.TrimSuffix(r.Path, ".container")
			}
			container.Name = name
			volumes, err := parseContainerFileForVolumes(a.Repo, comp, unitfile)
			if err != nil {
				return nil, err
			}
			container.Volumes = volumes
			newTarget.Containers = append(newTarget.Containers, container)
		}
		if r.Path == "MANIFEST.toml" {
			newTarget.Manifest, err = a.Repo.GetManifest(comp)
			if err != nil {
				return nil, err
			}
		}
	}
	if newTarget.Manifest == nil {
		return nil, fmt.Errorf("component %v is invalid", comp.Name)
	}
	return &newTarget, nil
}

func parseContainerFileForVolumes(repo *repository.HostComponentRepository, parent *components.Component, unitfile *parser.UnitFile) (map[string]containers.Volume, error) {
	results := make(map[string]containers.Volume)
	volumes := unitfile.LookupAll("Container", "Volume")
	for _, v := range volumes {
		vs := strings.Split(v, ":")
		if len(vs) < 2 {
			return nil, fmt.Errorf("volume %v is in invalid format", v)
		}
		volFile := vs[0]
		if strings.HasSuffix(volFile, ".volume") {
			log.Infof("parsing volume %v", volFile)
			path, err := repo.GetResource(parent, volFile)
			if err != nil {
				return nil, err
			}
			volumeFileData, err := repo.ReadResource(path)
			if err != nil {
				return nil, err
			}
			volumeUnitFile := parser.NewUnitFile()
			err = volumeUnitFile.Parse(volumeFileData)
			if err != nil {
				return nil, err
			}
			volumeName, found := volumeUnitFile.Lookup("Container", "VolumeName")
			if !found {
				volumeName = fmt.Sprintf("systemd-%v", strings.TrimSuffix(volFile, ".volume"))
			}
			results[strings.TrimSuffix(volFile, ".volume")] = containers.Volume{
				Name: volumeName,
			}
		}
	}
	return results, nil
}

func (a *Athanor) BackupTarget(ctx context.Context, t *ComponentTarget) error {
	if t.Manifest.Backups == nil {
		return nil
	}
	conf := t.Manifest.Backups
	pausedContainers := []string{}
	stoppedServices := []string{}

	defer func() {
		for _, s := range stoppedServices {
			log.Info("starting service", "service", s)
			err := a.sm.Apply(ctx, s, services.ServiceStart)
			if err != nil {
				log.Warn("error starting service", "service", s, "error", err)
			}
		}
	}()
	defer func() {
		for _, c := range pausedContainers {
			log.Info("unpausing container", "container", c)
			err := a.pm.UnpauseContainer(ctx, c)
			if err != nil {
				log.Warn("error unpausing container", "container", c, "error", err)
			}
		}
	}()
	if conf.Pause {
		for _, c := range t.Containers {
			log.Info("pausing container", "container", c.Name)
			err := a.pm.PauseContainer(ctx, c.Name)
			if err != nil {
				return fmt.Errorf("error pausing container %v: %w", c.Name, err)
			}
			pausedContainers = append(pausedContainers, c.Name)
		}
	} else if !conf.Online {
		for _, s := range t.Manifest.Services {
			liveService, err := a.sm.Get(ctx, s.Service)
			if err != nil {
				return fmt.Errorf("error getting service %v: %w", s.Service, err)
			}
			if liveService.State == "active" && strings.HasSuffix(liveService.Name, ".service") {
				log.Info("stopping service", "service", s.Service)
				err := a.sm.Apply(ctx, s.Service, services.ServiceStop)
				if err != nil {
					return fmt.Errorf("error stopping service %v: %w", s.Service, err)
				}
				stoppedServices = append(stoppedServices, s.Service)
			}
		}
	}
	for _, c := range t.Containers {
		for _, v := range c.Volumes {
			if slices.ContainsFunc(conf.Skip, func(skippedVol string) bool {
				return skippedVol == v.Name
			}) {
				log.Info("skipping configured volume %v/%v", c, v)
				continue
			}
			log.Infof("dumping volume %v", v.Name)
			err := a.pm.DumpVolume(ctx, v, a.Destination, !conf.NoCompress)
			if err != nil {
				return fmt.Errorf("error dumping volume %v: %w", v, err)
			}
		}
	}
	return nil
}
