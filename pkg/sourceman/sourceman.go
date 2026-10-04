package sourceman

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"charm.land/log/v2"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/repository"
	"primamateria.systems/materia/pkg/source"
	"primamateria.systems/materia/pkg/source/git"
	"primamateria.systems/materia/pkg/source/local"
	"primamateria.systems/materia/pkg/source/oci"
)

type SourceManConfig struct {
	SourceDir, RemoteDir string
}

type sourcePlan struct {
	source.Source
	Primary bool
	Opts    *source.SyncOpts
	Report  *source.SyncReport
}

type srcMaker func(r manifests.RemoteComponentConfig, dest string) (source.Source, error)

type SourceManager struct {
	components.ComponentReader
	maker          srcMaker
	sourceDir      string
	remoteRegistry *repository.RemoteComponentRegistry
	sources        []sourcePlan
}

func NewSourceManager(c *SourceManConfig) (*SourceManager, error) {
	registry, err := repository.NewRemoteComponentRegistry(c.RemoteDir)
	if err != nil {
		return nil, err
	}
	repo, err := repository.NewSourceComponentRepository(c.SourceDir, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to create source component repo: %w", err)
	}
	return &SourceManager{
		ComponentReader: repo,
		sourceDir:       c.SourceDir,
		remoteRegistry:  registry,
		maker:           DefaultSourceMaker,
	}, nil
}

func (s *SourceManager) Sync(ctx context.Context, opts *source.SyncOpts) error {
	for i, src := range s.sources {
		o := &source.SyncOpts{}
		if src.Opts != nil {
			o = src.Opts
		} else if opts != nil {
			o = opts
		}
		report, err := src.Sync(ctx, *o)
		if err != nil {
			return fmt.Errorf("error syncing source: %w", err)
		}
		src.Report = report
		s.sources[i] = src
	}
	return nil
}

func (s *SourceManager) Rollback(ctx context.Context) error {
	for _, src := range s.sources {
		if !src.Inspect().SupportsRollback {
			return fmt.Errorf("unable to rollback: unsupported source type: %v", src.Source)
		}
	}
	for i, src := range s.sources {
		if src.Report == nil {
			return fmt.Errorf("unable to rollback: no plan for %v", src.Source)
		}
		r := src.Report
		if r.OldRevision == "" && src.Primary {
			return fmt.Errorf("unable to rollback primary repository to nothing")
		}
		o := source.SyncOpts{
			Revision: r.OldRevision,
		}
		if src.Opts != nil {
			o.Subpath = src.Opts.Subpath
		}
		log.Info("rolling back", "source", src.Source, "revision", r.OldRevision)
		if r.OldRevision == "" {
			err := src.Clean()
			if err != nil {
				return err
			}
		} else {
			report, err := src.Sync(ctx, o)
			if err != nil {
				return fmt.Errorf("unable to rollback %v: %w", src.Source, err)
			}
			src.Report = report
			s.sources[i] = src
		}
	}
	return nil
}

func (s *SourceManager) AddSource(newSource source.Source, opts *source.SyncOpts, report *source.SyncReport, primary bool) error {
	s.sources = append(s.sources, sourcePlan{newSource, primary, opts, report})
	return nil
}

func (s *SourceManager) LoadManifest(filename string) (*manifests.MateriaManifest, error) {
	manifestLocation := filepath.Join(s.sourceDir, filename)
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
	s.remoteRegistry.Reset()
	s.sources = slices.DeleteFunc(s.sources, func(p sourcePlan) bool { return !p.Primary })
	remoteKeys := slices.Sorted(maps.Keys(man.Remotes))
	for _, name := range remoteKeys {
		r := man.Remotes[name]
		if ok, _ := s.ComponentExists(name); ok {
			log.Debugf("loading remote component that's shadowed by a local component: %v", name)
		}
		localpath, err := s.remoteRegistry.ClonePath(name)
		if err != nil {
			return fmt.Errorf("invalid remote %v: %w", name, err)
		}
		remoteSource, err := s.maker(r, localpath)
		if err != nil {
			return fmt.Errorf("unable to construct remote %v: %w", name, err)
		}

		if r.Subpath != "" && !filepath.IsLocal(r.Subpath) {
			return fmt.Errorf("invalid subpath %q", r.Subpath)
		}
		// Do initial sync here since we need the repository manifest downloaded before loading the remotes
		// and will thus miss the initial Sync() call
		report, err := remoteSource.Sync(ctx, source.SyncOpts{
			Subpath:  r.Subpath,
			Revision: r.Revision,
		})
		if err != nil {
			return err
		}
		if err := s.remoteRegistry.Register(name, r.Subpath); err != nil {
			return err
		}
		if err := s.AddSource(remoteSource, &source.SyncOpts{
			Revision: r.Revision,
			Subpath:  r.Subpath,
		}, report, false); err != nil {
			return fmt.Errorf("unable to add remote component source %v: %w", name, err)
		}

	}
	return s.remoteRegistry.Prune()
}

func (s *SourceManager) Clean() error {
	return s.ComponentReader.Clean()
}

func DefaultSourceMaker(r manifests.RemoteComponentConfig, localpath string) (source.Source, error) {
	var remoteSource source.Source
	var err error

	if r.GitSource != nil {
		r.GitSource.LocalRepository = localpath
		remoteSource, err = git.NewGitSource(r.GitSource)
		if err != nil {
			return nil, fmt.Errorf("invalid git source: %w", err)
		}
	}
	if r.FileSource != nil {
		r.FileSource.Destination = localpath
		remoteSource, err = local.NewLocalFileSource(r.FileSource)
		if err != nil {
			return nil, fmt.Errorf("invalid file source: %w", err)
		}
	}
	if r.OciSource != nil {
		r.OciSource.LocalRepository = localpath
		remoteSource, err = oci.NewOCISource(r.OciSource)
		if err != nil {
			return nil, fmt.Errorf("invalid oci source: %w", err)
		}
	}
	if remoteSource == nil {
		return nil, fmt.Errorf("no valid source config")
	}
	return remoteSource, nil
}
