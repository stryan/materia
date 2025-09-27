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
	"primamateria.systems/materia/internal/attributes/age"
	"primamateria.systems/materia/internal/attributes/mem"
	"primamateria.systems/materia/internal/attributes/sops"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/facts"
	"primamateria.systems/materia/internal/manifests"
	"primamateria.systems/materia/internal/materia"
	"primamateria.systems/materia/internal/repository"

	fileattrs "primamateria.systems/materia/internal/attributes/file"
	"primamateria.systems/materia/internal/services"
	"primamateria.systems/materia/internal/source/git"

	filesource "primamateria.systems/materia/internal/source/file"
)

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
	if c.UseStdout {
		log.Default().SetOutput(os.Stdout)
	}
	if c.Debug {
		log.Default().SetLevel(log.DebugLevel)
		log.Default().SetReportCaller(true)
	}
	err = os.Mkdir(filepath.Join(c.MateriaDir, "materia"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating prefix: %w", err)
	}
	err = os.Mkdir(c.OutputDir, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating output dir: %w", err)
	}
	err = os.Mkdir(c.SourceDir, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating source repo: %w", err)
	}
	err = os.Mkdir(filepath.Join(c.MateriaDir, "materia", "components"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating components in prefix: %w", err)
	}
	sourceConfig, err := materia.NewSourceConfig(k.Cut("source"))
	if err != nil {
		return nil, err
	}
	err = sourceConfig.Validate()
	if err != nil {
		log.Fatal(err)
	}
	var source materia.Source

	parsedPath := strings.Split(sourceConfig.URL, "://")
	switch parsedPath[0] {
	case "git":
		config, err := git.NewConfig(k, c.SourceDir, parsedPath[1])
		if err != nil {
			return nil, fmt.Errorf("error creating git config: %w", err)
		}
		source, err = git.NewGitSource(config)
		if err != nil {
			return nil, fmt.Errorf("invalid git source: %w", err)
		}
	case "file":
		config, err := filesource.NewConfig(k, c.SourceDir, parsedPath[1])
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
	// Ensure local cache
	if sourceConfig.NoSync {
		log.Debug("skipping cache update on request")
	} else {
		log.Debug("updating configured source cache")
		err = source.Sync(ctx)
		if err != nil {
			return nil, fmt.Errorf("error syncing source: %w", err)
		}
	}
	// load manifest

	manifestLocation := filepath.Join(c.SourceDir, "MANIFEST.toml")
	man, err := manifests.LoadMateriaManifest(manifestLocation)
	if err != nil {
		return nil, fmt.Errorf("error loading manifest: %w", err)
	}
	if err := man.Validate(); err != nil {
		return nil, fmt.Errorf("invalid materia manifest: %w", err)
	}

	// reparse config with new data from manifest

	sm, err := services.NewServices(ctx, &services.ServicesConfig{
		Timeout: c.Timeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	cm, err := containers.NewPodmanManager()
	if err != nil {
		log.Fatal(err)
	}

	scriptRepo, err := repository.NewFileRepository(c.ScriptsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create script repo: %w", err)
	}
	serviceRepo, err := repository.NewFileRepository(c.ServiceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create service repo: %w", err)
	}
	sourceRepo, err := repository.NewSourceComponentRepository(filepath.Join(c.SourceDir, "components"))
	if err != nil {
		return nil, fmt.Errorf("failed to create source component repo: %w", err)
	}
	hostRepo, err := repository.NewHostComponentRepository(c.QuadletDir, filepath.Join(c.MateriaDir, "materia", "components"))
	if err != nil {
		return nil, fmt.Errorf("failed to create host component repo: %w", err)
	}
	var attributesEngine materia.AttributesEngine
	// TODO replace this with attributes chaining
	switch c.Attributes {
	case "age":
		ageConfig, err := age.NewConfig(k)
		if err != nil {
			return nil, fmt.Errorf("error creating age config: %w", err)
		}
		attributesEngine, err = age.NewAgeStore(*ageConfig, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating age store: %w", err)
		}
	case "file":
		fileConfig, err := fileattrs.NewConfig(k)
		if err != nil {
			return nil, fmt.Errorf("error creating file config: %w", err)
		}
		attributesEngine, err = fileattrs.NewFileStore(*fileConfig, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating file store: %w", err)
		}
	case "sops":
		sopsConfig, err := sops.NewConfig(k)
		if err != nil {
			return nil, fmt.Errorf("error creating sops config: %w", err)
		}
		attributesEngine, err = sops.NewSopsStore(*sopsConfig, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating sops store: %w", err)
		}
	case "mem":
		attributesEngine = mem.NewMemoryEngine()
	default:
		return nil, fmt.Errorf("failed to initialize attributes manager: invalid type")
	}
	log.Debug("loading host facts")
	factsm, err := facts.NewHostFacts(ctx, c.Hostname)
	if err != nil {
		return nil, fmt.Errorf("error generating facts: %w", err)
	}

	m, err := materia.NewMateria(ctx, c, source, man, factsm, attributesEngine, sm, cm, scriptRepo, serviceRepo, sourceRepo, hostRepo)
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
			strings.TrimPrefix(s, "MATERIA_")), "_", ".", 1)
	}), nil)
	if err != nil {
		return nil, fmt.Errorf("error loading config from env: %w", err)
	}
	err = cliConf.Load(confmap.Provider(cliflags, "."), nil)
	if err != nil {
		return nil, err
	}
	// merge non-manifest confs
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

func doctorSetup(ctx context.Context, configFile string, cliflags map[string]any) (*materia.Materia, error) {
	k, err := LoadConfigs(ctx, configFile, cliflags)
	if err != nil {
		return nil, err
	}

	c, err := materia.NewConfig(k)
	if err != nil {
		log.Fatal(err)
	}
	err = c.Validate()
	if err != nil {
		log.Fatal(err)
	}
	if c.UseStdout {
		log.Default().SetOutput(os.Stdout)
	}
	if c.Debug {
		log.Default().SetLevel(log.DebugLevel)
		log.Default().SetReportCaller(true)
	}
	sm, err := services.NewServices(ctx, &services.ServicesConfig{
		Timeout: c.Timeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	cm, err := containers.NewPodmanManager()
	if err != nil {
		log.Fatal(err)
	}

	hostRepo, err := repository.NewHostComponentRepository(c.QuadletDir, filepath.Join(c.MateriaDir, "materia", "components"))
	if err != nil {
		return nil, fmt.Errorf("failed to create host component repo: %w", err)
	}

	// log.Debug("loading manifest")
	// manifestLocation := filepath.Join(c.SourceDir, "MANIFEST.toml")
	// man, err := manifests.LoadMateriaManifest(manifestLocation)
	// if err != nil {
	// 	return nil, fmt.Errorf("error loading manifest: %w", err)
	// }
	// if err := man.Validate(); err != nil {
	// 	return nil, fmt.Errorf("invalid materia manifest: %w", err)
	// }
	// err = k.Load(file.Provider(manifestLocation), toml.Parser())
	// if err != nil {
	// 	return nil, err
	// }

	m, err := materia.NewMateria(ctx, c, nil, nil, nil, nil, sm, cm, nil, nil, nil, hostRepo)
	if err != nil {
		log.Fatal(err)
	}
	return m, nil
}
