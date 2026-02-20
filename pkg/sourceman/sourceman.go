package sourceman

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
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

type SourceManager struct {
	components.ComponentReader
	sourceDir string
	remoteDir string
	sources   []source.Source
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

func (s *SourceManager) Sync(ctx context.Context) error {
	for _, src := range s.sources {
		err := src.Sync(ctx)
		if err != nil {
			return fmt.Errorf("error syncing source: %w", err)
		}
	}
	return nil
}

func (s *SourceManager) AddSource(newSource source.Source) error {
	s.sources = append(s.sources, newSource)

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

func (s *SourceManager) SyncRemotes(ctx context.Context) error {
	manifestLocation := filepath.Join(s.sourceDir, manifests.MateriaManifestFile)
	man, err := manifests.LoadMateriaManifest(manifestLocation)
	if err != nil {
		return err
	}
	for name, r := range man.Remotes {
		var remoteSource source.Source
		if r.GitSource != nil {
			remoteSource, err = git.NewGitSource(r.GitSource)
			if err != nil {
				return fmt.Errorf("invalid git source: %w", err)
			}
		}
		if r.FileSource != nil {
			remoteSource, err = file.NewFileSource(r.FileSource)
			if err != nil {
				return fmt.Errorf("invalid file source: %w", err)
			}
		}
		if r.OciSource != nil {
			remoteSource, err = oci.NewOCISource(r.OciSource)
			if err != nil {
				return fmt.Errorf("invalid oci source: %w", err)
			}
		}
		if remoteSource == nil {
			return fmt.Errorf("remote %v has no valid source config", name)
		}
		localpath := filepath.Join(s.remoteDir, "components", name)
		if err := remoteSource.Sync(ctx); err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(localpath, manifests.ComponentManifestFile)); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("invalid remote component %v", err)
			}
			return fmt.Errorf("cannot determine remote component validity: %w", err)
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
