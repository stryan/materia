package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type FileSource struct {
	repo string
	path string
}

func (f *FileSource) Close(_ context.Context) (_ error) {
	return nil
}

func (f *FileSource) Clean() (_ error) {
	return os.RemoveAll(f.path)
}

func NewFileSource(path, repo string) (*FileSource, error) {
	if _, err := os.Stat(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		err = os.Mkdir(path, 0o755)
		if err != nil {
			return nil, err
		}
	}
	return &FileSource{repo, path}, nil
}

func (f *FileSource) Sync(ctx context.Context) error {
	if _, err := os.Stat(f.path); os.IsNotExist(err) {
		return fmt.Errorf("source destination path %v does not exist", f.path)
	}
	if _, err := os.Stat(f.repo); os.IsNotExist(err) {
		return fmt.Errorf("source repo %v does not exist", f.repo)
	}
	entries, err := os.ReadDir(f.path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(f.path, e.Name())); err != nil {
			return fmt.Errorf("error syncing filesystem: can't clear path: %w", err)
		}
	}

	repoFS := os.DirFS(f.repo)
	err = os.CopyFS(f.path, repoFS)
	if err != nil {
		return fmt.Errorf("error syncing filesystem: can't copy fs: %w", err)
	}
	return nil
}
