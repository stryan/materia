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
	"primamateria.systems/materia/internal/materia"
	"primamateria.systems/materia/internal/source"

	"primamateria.systems/materia/internal/source/git"

	filesource "primamateria.systems/materia/internal/source/file"
)

func setupDirectories(c *materia.MateriaConfig) error {
	err := os.Mkdir(filepath.Join(c.MateriaDir, "materia"), 0o755)
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
	err = os.Mkdir(filepath.Join(c.MateriaDir, "materia", "components"), 0o755)
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

func syncLocalRepo(ctx context.Context, k *koanf.Koanf, sourceDir string) error {
	rawSourceConfig := k.Cut("source")
	var sourceConfig source.SourceConfig
	sourceConfig.URL = rawSourceConfig.String("url")
	noSync := rawSourceConfig.Bool("no_sync")

	err := sourceConfig.Validate()
	if err != nil {
		log.Fatal(err)
	}
	var source materia.Source

	parsedPath := strings.Split(sourceConfig.URL, "://")
	switch parsedPath[0] {
	case "git":
		config, err := git.NewConfig(k, sourceDir, parsedPath[1])
		if err != nil {
			return fmt.Errorf("error creating git config: %w", err)
		}
		source, err = git.NewGitSource(config)
		if err != nil {
			return fmt.Errorf("invalid git source: %w", err)
		}
	case "file":
		config, err := filesource.NewConfig(k, sourceDir, parsedPath[1])
		if err != nil {
			return fmt.Errorf("error creating file config: %w", err)
		}
		source, err = filesource.NewFileSource(config)
		if err != nil {
			return fmt.Errorf("invalid file source: %w", err)
		}
	default:
		return fmt.Errorf("invalid source: %v", parsedPath[0])
	}
	// Ensure local cache
	if noSync {
		log.Debug("skipping cache update on request")
	} else {
		log.Debug("updating configured source cache")
		err = source.Sync(ctx)
		if err != nil {
			return fmt.Errorf("error syncing source: %w", err)
		}
	}
	return nil
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
	if err := setupDirectories(c); err != nil {
		return nil, fmt.Errorf("error creating base directories: %w", err)
	}
	setupLogger(c)

	if err := syncLocalRepo(ctx, k, c.SourceDir); err != nil {
		return nil, err
	}
	hm, err := NewHostManager(c)
	if err != nil {
		return nil, err
	}

	m, err := materia.NewMateriaFromConfig(ctx, c, hm)
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
