package sourceman

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/internal/materia"
	"primamateria.systems/materia/internal/repository"
	"primamateria.systems/materia/internal/source/file"
	"primamateria.systems/materia/internal/source/git"
	"primamateria.systems/materia/pkg/manifests"
)

type SourceManager struct {
	materia.ComponentRepository
	sourceDir string
	remoteDir string
	sources   []materia.Source
}

func NewSourceManager(c *materia.MateriaConfig) (*SourceManager, error) {
	sourceRepo, err := repository.NewSourceComponentRepository(c.SourceDir, c.RemoteDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create source component repo: %w", err)
	}
	return &SourceManager{
		ComponentRepository: sourceRepo,
		sourceDir:           c.SourceDir,
		remoteDir:           c.RemoteDir,
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

func (s *SourceManager) AddSource(newSource materia.Source) error {
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
	// TODO update to new source format?
	for name, r := range man.Remotes {
		parsedPath := strings.Split(r.URL, "://")
		var remoteSource materia.Source
		var err error
		switch parsedPath[0] {
		case "git":
			localpath := filepath.Join(s.remoteDir, "components", name)
			remoteSource, err = git.NewGitSource(&git.Config{
				Branch:          r.Version,
				PrivateKey:      "",
				Username:        "",
				Password:        "",
				KnownHosts:      "",
				Insecure:        false,
				LocalRepository: localpath,
				URL:             r.URL,
			})
			if err != nil {
				return fmt.Errorf("invalid git source: %w", err)
			}
		case "file":
			localpath := filepath.Join(s.remoteDir, "components", name)
			remoteSource, err = file.NewFileSource(&file.Config{
				SourcePath:  r.URL,
				Destination: localpath,
			})
			if err != nil {
				return fmt.Errorf("invalid file source: %w", err)
			}
		default:
			return fmt.Errorf("invalid source: %v", parsedPath[0])
		}
		if err := remoteSource.Sync(ctx); err != nil {
			return err
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
