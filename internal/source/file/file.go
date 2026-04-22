package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"primamateria.systems/materia/pkg/source"
)

type FileSource struct {
	RemoteRepository string
	Destination      string
}

func (f *FileSource) Close(_ context.Context) (_ error) {
	return nil
}

func (f *FileSource) Clean() (_ error) {
	return os.RemoveAll(f.Destination)
}

func NewFileSource(c *Config) (*FileSource, error) {
	source := strings.TrimPrefix(c.SourcePath, "file://")
	if _, err := os.Stat(source); err != nil {
		return nil, err
	}
	return &FileSource{
		RemoteRepository: source,
		Destination:      c.Destination,
	}, nil
}

func (f *FileSource) Sync(ctx context.Context, opts source.SyncOpts) error {
	if _, err := os.Stat(f.Destination); os.IsNotExist(err) {
		return fmt.Errorf("source destination path %v does not exist", f.Destination)
	}
	if opts.Subpath != "" {
		if _, err := os.Stat(filepath.Join(f.RemoteRepository, opts.Subpath)); os.IsNotExist(err) {
			return fmt.Errorf("source subpath %v/%v does not exist", f.RemoteRepository, opts.Subpath)
		}
	}
	entries, err := os.ReadDir(f.Destination)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(f.Destination, e.Name())); err != nil {
			return fmt.Errorf("error syncing filesystem: can't clear path: %w", err)
		}
	}
	source := filepath.Join(f.RemoteRepository, opts.Subpath)

	repoFS := os.DirFS(source)
	err = os.CopyFS(f.Destination, repoFS)
	if err != nil {
		return fmt.Errorf("error syncing filesystem: can't copy fs: %w", err)
	}
	return nil
}
