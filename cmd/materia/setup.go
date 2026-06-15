package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/internal/config"
	"primamateria.systems/materia/internal/materia"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/hostman"
	"primamateria.systems/materia/pkg/source"

	"primamateria.systems/materia/pkg/sourceman"

	"primamateria.systems/materia/internal/source/git"
	"primamateria.systems/materia/internal/source/oci"

	filesource "primamateria.systems/materia/internal/source/file"
)

func setupDirectories(c *materia.MateriaConfig) error {
	err := os.Mkdir(filepath.Join(c.MateriaDir), 0o755)
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

func setupLogger(c *materia.MateriaConfig) {
	if c.UseStdout {
		log.Default().SetOutput(os.Stdout)
	}
	if c.Debug {
		log.Default().SetLevel(log.DebugLevel)
		log.Default().SetReportCaller(true)
	}
}

func getLocalRepo(k *koanf.Koanf, sourceDir string) (source.Source, error) {
	rawSourceConfig := k.Cut("source")
	var sourceConfig source.SourceConfig
	sourceConfig.URL = rawSourceConfig.String("url")
	sourceConfig.Kind = rawSourceConfig.String("kind")

	err := sourceConfig.Validate()
	if err != nil {
		return nil, err
	}
	var source source.Source
	switch sourceConfig.Kind {
	case "git":
		config, err := git.NewConfig(k, sourceDir, sourceConfig.URL)
		if err != nil {
			return nil, fmt.Errorf("error creating git config: %w", err)
		}
		source, err = git.NewGitSource(config)
		if err != nil {
			return nil, fmt.Errorf("invalid git source: %w", err)
		}
	case "file":
		config, err := filesource.NewConfig(k, sourceDir, sourceConfig.URL)
		if err != nil {
			return nil, fmt.Errorf("error creating file config: %w", err)
		}
		source, err = filesource.NewFileSource(config)
		if err != nil {
			return nil, fmt.Errorf("invalid file source: %w", err)
		}
	case "oci":
		config, err := oci.NewConfig(k, sourceDir, sourceConfig.URL)
		if err != nil {
			return nil, fmt.Errorf("error creating OCI config: %w", err)
		}
		source, err = oci.NewOCISource(config)
		if err != nil {
			return nil, fmt.Errorf("invalid OCI source: %w", err)
		}
	default:
		// try to guess from URL
		log.Warn("guessing source type via URL is deprecated and will be removed in 0.7 . Please provide a source.kind setting")
		parsedPath := strings.Split(sourceConfig.URL, "://")
		switch parsedPath[0] {
		case "git":
			config, err := git.NewConfig(k, sourceDir, sourceConfig.URL)
			if err != nil {
				return nil, fmt.Errorf("error creating git config: %w", err)
			}
			source, err = git.NewGitSource(config)
			if err != nil {
				return nil, fmt.Errorf("invalid git source: %w", err)
			}
		case "file":
			config, err := filesource.NewConfig(k, sourceDir, sourceConfig.URL)
			if err != nil {
				return nil, fmt.Errorf("error creating file config: %w", err)
			}
			source, err = filesource.NewFileSource(config)
			if err != nil {
				return nil, fmt.Errorf("invalid file source: %w", err)
			}
		case "oci":
			config, err := oci.NewConfig(k, sourceDir, sourceConfig.URL)
			if err != nil {
				return nil, fmt.Errorf("error creating OCI config: %w", err)
			}
			source, err = oci.NewOCISource(config)
			if err != nil {
				return nil, fmt.Errorf("invalid OCI source: %w", err)
			}
		default:
			return nil, fmt.Errorf("invalid source: %v", parsedPath[0])
		}

	}
	return source, nil
}

func setup(ctx context.Context, configFile string, cliflags map[string]any) (*materia.Materia, error) {
	k, err := config.LoadConfigs(ctx, configFile, cliflags)
	if err != nil {
		return nil, fmt.Errorf("error generating config blob: %w", err)
	}
	c, err := materia.NewConfig(k)
	if err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}
	err = c.Validate()
	if err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}
	setupLogger(c)
	log.Debug("config loaded")
	if err := setupDirectories(c); err != nil {
		return nil, fmt.Errorf("error creating base directories: %w", err)
	}

	mainRepo, err := getLocalRepo(k, c.SourceDir)
	if err != nil {
		return nil, err
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
	sm, err := sourceman.NewSourceManager(smc)
	if err != nil {
		return nil, err
	}
	log.Debug("adding source", "source", mainRepo)
	err = sm.AddSource(mainRepo, nil, nil, true)
	if err != nil {
		return nil, err
	}
	if !c.NoSync {
		log.Debug("syncing source")
		err = sm.Sync(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("error with initial repo sync: %w", err)
		}
		log.Debug("loading remotes")
		err = sm.LoadRemotes(ctx)
		if err != nil {
			return nil, fmt.Errorf("error with repo remotes load: %w", err)
		}
	}
	hm, err := hostman.NewHostManager(ctx, hmc)
	if err != nil {
		return nil, err
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
			}
		}
		if materiaContainer != nil {
			if dataSrc, ok := materiaContainer.BindMounts[materia.DefaultDataDir]; ok {
				c.ExecutorConfig.MateriaDir = dataSrc.Source
			}
			if quadSrc, ok := materiaContainer.BindMounts[materia.DefaultQuadletDir]; ok {
				c.ExecutorConfig.QuadletDir = quadSrc.Source
			}
			if scriptSrc, ok := materiaContainer.BindMounts[materia.DefaultScriptsDir]; ok {
				c.ExecutorConfig.ScriptsDir = scriptSrc.Source
			}
			if serviceSrc, ok := materiaContainer.BindMounts[materia.DefaultServiceDir]; ok {
				c.ExecutorConfig.ServiceDir = serviceSrc.Source
			}

		}
	}
	m, err := materia.NewMateriaFromConfig(ctx, c, hm, sm)
	if err != nil {
		log.Fatal(err)
	}
	return m, nil
}
