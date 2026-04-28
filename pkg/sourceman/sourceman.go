package sourceman

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/log/v2"
	"primamateria.systems/materia/internal/repository"
	"primamateria.systems/materia/internal/source/file"
	"primamateria.systems/materia/internal/source/git"
	"primamateria.systems/materia/internal/source/oci"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/source"
)

type SourceManConfig struct {
	SourceDir, RemoteDir string
}

type sourcePlan struct {
	source.Source
	Opts source.SyncOpts
}

type SourceManager struct {
	components.ComponentReader
	sourceDir string
	remoteDir string
	sources   []sourcePlan
}

func NewSourceManager(c *SourceManConfig) (*SourceManager, error) {
	sourceRepo, err := repository.NewSourceComponentRepository(c.SourceDir, c.RemoteDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create source component repo: %w", err)
	}
	return &SourceManager{
		ComponentReader: sourceRepo,
		sourceDir:       c.SourceDir,
		remoteDir:       c.RemoteDir,
	}, nil
}

func (s *SourceManager) Sync(ctx context.Context, opts *source.SyncOpts) error {
	for _, src := range s.sources {
		o := src.Opts
		if opts != nil {
			o = *opts
		}
		err := src.Sync(ctx, o)
		if err != nil {
			return fmt.Errorf("error syncing source: %w", err)
		}
	}
	return nil
}

func (s *SourceManager) AddSource(newSource source.Source, opts *source.SyncOpts) error {
	if opts != nil {
		s.sources = append(s.sources, sourcePlan{newSource, *opts})
	} else {
		s.sources = append(s.sources, sourcePlan{newSource, source.SyncOpts{}})
	}
	return nil
}

func (s *SourceManager) LoadManifest(filename string) (*manifests.MateriaManifest, error) {
	manifestLocation := filepath.Join(s.sourceDir, manifests.MateriaManifestFile)
	man, err := manifests.LoadMateriaManifest(manifestLocation)
	if err != nil {
		return nil, fmt.Errorf("error loading manifest: %w", err)
	}
	return man, nil
}

func (s *SourceManager) LoadRemotes(ctx context.Context) error {
	manifestLocation := filepath.Join(s.sourceDir, manifests.MateriaManifestFile)
	man, err := manifests.LoadMateriaManifest(manifestLocation)
	if err != nil {
		return err
	}
	for name, r := range man.Remotes {
		var remoteSource source.Source
		localpath := filepath.Join(s.remoteDir, "components", name)
		if r.GitSource != nil {
			r.GitSource.LocalRepository = localpath
			remoteSource, err = git.NewGitSource(r.GitSource)
			if err != nil {
				return fmt.Errorf("invalid git source: %w", err)
			}
		}
		if r.FileSource != nil {
			r.FileSource.Destination = localpath
			remoteSource, err = file.NewFileSource(r.FileSource)
			if err != nil {
				return fmt.Errorf("invalid file source: %w", err)
			}
		}
		if r.OciSource != nil {
			r.OciSource.LocalRepository = localpath
			remoteSource, err = oci.NewOCISource(r.OciSource)
			if err != nil {
				return fmt.Errorf("invalid oci source: %w", err)
			}
		}
		if remoteSource == nil {
			return fmt.Errorf("remote %v has no valid source config", name)
		}
		// Do initial sync here since we need the repository manifest downloaded before loading the remotes
		// and will thus miss the initial Sync() call
		if err := remoteSource.Sync(ctx, source.SyncOpts{
			Subpath: r.Subpath,
		}); err != nil {
			return err
		}
		if r.Subpath != "" {
			localpath = filepath.Join(localpath, r.Subpath)
		}
		if _, err := os.Stat(filepath.Join(localpath, manifests.ComponentManifestFile)); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("invalid remote component %v", err)
			}
			return fmt.Errorf("cannot determine remote component validity: %w", err)
		}
		if err := s.AddSource(remoteSource, &source.SyncOpts{
			Subpath: r.Subpath,
		}); err != nil {
			return fmt.Errorf("unable to add remote component source %v: %w", name, err)
		}

	}
	// remove old remote components to keep things tidy
	entries, err := os.ReadDir(filepath.Join(s.remoteDir, "components"))
	if err != nil {
		return err
	}
	for _, v := range entries {
		if v.IsDir() {
			if _, ok := man.Remotes[v.Name()]; !ok {
				log.Debugf("Removing old remote component %v", v.Name())
				err := os.RemoveAll(filepath.Join(s.remoteDir, "components", v.Name()))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *SourceManager) Clean() error {
	// TODO
	return nil
}
