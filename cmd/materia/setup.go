package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/materia"
	"primamateria.systems/materia/internal/source"
	"primamateria.systems/materia/pkg/hostman"
	"primamateria.systems/materia/pkg/sourceman"

	"primamateria.systems/materia/internal/source/git"

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

func getLocalRepo(k *koanf.Koanf, sourceDir string) (materia.Source, error) {
	rawSourceConfig := k.Cut("source")
	var sourceConfig source.SourceConfig
	sourceConfig.URL = rawSourceConfig.String("url")
	sourceConfig.Kind = rawSourceConfig.String("kind")

	err := sourceConfig.Validate()
	if err != nil {
		return nil, err
	}
	var source materia.Source

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
	default:
		// try to guess from URL
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
		default:
			return nil, fmt.Errorf("invalid source: %v", parsedPath[0])
		}

	}
	return source, nil
}

func setup(ctx context.Context, configFile string, cliflags map[string]any) (*materia.Materia, error) {
	k, err := LoadConfigs(ctx, configFile, cliflags)
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
	sm, err := sourceman.NewSourceManager(c)
	if err != nil {
		return nil, err
	}
	log.Debug("adding source", "source", mainRepo)
	err = sm.AddSource(mainRepo)
	if err != nil {
		return nil, err
	}
	if !c.NoSync {
		log.Debug("syncing source")
		err = sm.Sync(ctx)
		if err != nil {
			return nil, fmt.Errorf("error with initial repo sync: %w", err)
		}
		log.Debug("syncing remotes")
		err = sm.SyncRemotes(ctx)
		if err != nil {
			return nil, fmt.Errorf("error with repo remotes sync: %w", err)
		}
	}
	hm, err := hostman.NewHostManager(ctx, c)
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

func LoadConfigs(_ context.Context, configFile string, cliflags map[string]any) (*koanf.Koanf, error) {
	k := koanf.New(".")
	fileConf := koanf.New(".")
	envConf := koanf.New(".")
	cliConf := koanf.New(".")
	if configFile != "" {
		err := fileConf.Load(file.Provider(configFile), toml.Parser())
		if err != nil {
			return nil, fmt.Errorf("error loading config file: %w", err)
		}
	}
	err := envConf.Load(env.Provider("MATERIA", ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, "MATERIA_")), "__", ".", 1)
	}), nil)
	if err != nil {
		return nil, fmt.Errorf("error loading config from env: %w", err)
	}
	err = cliConf.Load(confmap.Provider(cliflags, "."), nil)
	if err != nil {
		return nil, err
	}
	err = k.Merge(fileConf)
	if err != nil {
		return nil, fmt.Errorf("error building config: %w", err)
	}
	err = k.Merge(envConf)
	if err != nil {
		return nil, fmt.Errorf("error building config: %w", err)
	}
	err = k.Merge(cliConf)
	if err != nil {
		return nil, fmt.Errorf("error building config: %w", err)
	}

	return k, err
}
