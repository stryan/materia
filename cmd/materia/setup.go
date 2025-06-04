package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/containers"
	"git.saintnet.tech/stryan/materia/internal/facts"
	"git.saintnet.tech/stryan/materia/internal/manifests"
	"git.saintnet.tech/stryan/materia/internal/materia"
	"git.saintnet.tech/stryan/materia/internal/repository"
	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	"git.saintnet.tech/stryan/materia/internal/secrets/mem"
	"git.saintnet.tech/stryan/materia/internal/services"
	"git.saintnet.tech/stryan/materia/internal/source/file"
	"git.saintnet.tech/stryan/materia/internal/source/git"
	"github.com/charmbracelet/log"
)

func setup(ctx context.Context, c *materia.Config) (*materia.Materia, error) {
	err := c.Validate()
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
	var source materia.Source
	parsedPath := strings.Split(c.SourceURL, "://")
	switch parsedPath[0] {
	case "git":
		source, err = git.NewGitSource(c.SourceDir, parsedPath[1], c.GitConfig)
		if err != nil {
			return nil, fmt.Errorf("invalid git source: %w", err)
		}
	case "file":
		source, err = file.NewFileSource(c.SourceDir, parsedPath[1])
		if err != nil {
			return nil, fmt.Errorf("invalid file source: %w", err)
		}
	default:
		return nil, fmt.Errorf("invalid source: %v", parsedPath[0])
	}
	// Ensure local cache
	if c.NoSync {
		log.Debug("skipping cache update on request")
	} else {
		log.Debug("updating configured source cache")
		err = source.Sync(ctx)
		if err != nil {
			return nil, fmt.Errorf("error syncing source: %w", err)
		}
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

	log.Debug("loading manifest")
	man, err := manifests.LoadMateriaManifest(filepath.Join(c.SourceDir, "MANIFEST.toml"))
	if err != nil {
		return nil, fmt.Errorf("error loading manifest: %w", err)
	}
	if err := man.Validate(); err != nil {
		return nil, fmt.Errorf("invalid materia manifest: %w", err)
	}
	var secretManager materia.SecretsManager
	switch man.Secrets {
	case "age":
		conf, ok := man.SecretsConfig.(age.Config)
		if !ok {
			return nil, errors.New("tried to create an age secrets manager but config was not for age")
		}
		conf.RepoPath = c.SourceDir
		if c.AgeConfig != nil {
			conf.Merge(c.AgeConfig)
		}
		secretManager, err = age.NewAgeStore(conf)
		if err != nil {
			return nil, fmt.Errorf("error creating age store: %w", err)
		}

	case "mem":
		secretManager = mem.NewMemoryManager()
	default:
		secretManager = mem.NewMemoryManager()
	}
	log.Debug("loading host facts")
	factsm, err := facts.NewHostFacts(ctx, c.Hostname)
	if err != nil {
		return nil, fmt.Errorf("error generating facts: %w", err)
	}

	m, err := materia.NewMateria(ctx, c, source, man, factsm, secretManager, sm, cm, scriptRepo, serviceRepo, sourceRepo, hostRepo)
	if err != nil {
		log.Fatal(err)
	}
	return m, nil
}
