package materia

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"charm.land/log/v2"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/hostman"
	"primamateria.systems/materia/pkg/source"
	"primamateria.systems/materia/pkg/sourceman"
)

func NewFromKoanf(ctx context.Context, k *koanf.Koanf, src source.Source) (*Materia, error) {
	c, err := NewConfig(k)
	if err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}
	return NewMateria2(ctx, c, src)
}

func NewMateria2(ctx context.Context, c *MateriaConfig, src source.Source) (*Materia, error) {
	SetupLogger(c)
	if err := SetupDirectories(c); err != nil {
		return nil, fmt.Errorf("error creating base directories: %w", err)
	}
	hmc := &hostman.HostmanConfig{
		Hostname:         c.Hostname,
		DataDir:          c.MateriaDir,
		QuadletDir:       c.QuadletDir,
		ScriptsDir:       c.ScriptsDir,
		ServicesDir:      c.ServiceDir,
		ContainersConfig: c.ContainersConfig,
		ServicesConfig:   c.ServicesConfig,
	}
	smc := &sourceman.SourceManConfig{
		SourceDir: c.SourceDir,
		RemoteDir: c.RemoteDir,
	}

	hm, err := hostman.NewHostManager(ctx, hmc)
	if err != nil {
		return nil, err
	}

	sm, err := sourceman.NewSourceManager(smc)
	if err != nil {
		return nil, err
	}
	if err := sm.AddSource(src, nil, nil, true); err != nil {
		return nil, err
	}
	if !c.NoSync {
		if err := sm.Sync(ctx, nil); err != nil {
			return nil, fmt.Errorf("error with initial repo sync: %w", err)
		}
		if err := sm.LoadRemotes(ctx); err != nil {
			return nil, fmt.Errorf("error with repo remotes load: %w", err)
		}
	}
	if c.Rootless {
		cn := hm.GetHostname()
		potentials, err := hm.ListContainers(ctx, containers.ContainerListFilter{})
		if err != nil {
			return nil, fmt.Errorf("passed rootless but unable to list materia containers: %w", err)
		}
		var materiaContainer *containers.Container
		for _, v := range potentials {
			if v.Hostname == cn {
				materiaContainer = v
				break
			}
		}
		if materiaContainer != nil {
			if dataSrc, ok := materiaContainer.BindMounts[DefaultDataDir]; ok {
				c.ExecutorConfig.MateriaDir = dataSrc.Source
			}
			if quadSrc, ok := materiaContainer.BindMounts[DefaultQuadletDir]; ok {
				c.ExecutorConfig.QuadletDir = quadSrc.Source
			}
			if scriptSrc, ok := materiaContainer.BindMounts[DefaultScriptsDir]; ok {
				c.ExecutorConfig.ScriptsDir = scriptSrc.Source
			}
			if serviceSrc, ok := materiaContainer.BindMounts[DefaultServiceDir]; ok {
				c.ExecutorConfig.ServiceDir = serviceSrc.Source
			}

		}
	}

	return NewMateriaFromConfig(ctx, c, hm, sm)
}

func SetupLogger(c *MateriaConfig) {
	if c.UseStdout {
		log.Default().SetOutput(os.Stdout)
	}
	if c.Debug {
		log.Default().SetLevel(log.DebugLevel)
		log.Default().SetReportCaller(true)
	}
}

func SetupDirectories(c *MateriaConfig) error {
	err := os.Mkdir(c.MateriaDir, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("error creating prefix: %w", err)
	}
	err = os.Mkdir(c.OutputDir, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("error creating output dir: %w", err)
	}
	err = os.Mkdir(c.SourceDir, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("error creating source repo: %w", err)
	}
	err = os.MkdirAll(filepath.Join(c.RemoteDir, "components"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("error creating source repo: %w", err)
	}
	err = os.Mkdir(filepath.Join(c.MateriaDir, "components"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("error creating components in prefix: %w", err)
	}
	return nil
}
