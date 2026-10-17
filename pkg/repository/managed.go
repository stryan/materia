package repository

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
	"primamateria.systems/materia/pkg/atomic"
)

type ManagedDir struct {
	prefix string
	mode   os.FileMode
}

func NewManagedDir(prefix string, mode os.FileMode) (*ManagedDir, error) {
	abs, err := filepath.Abs(prefix)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(abs, 0o755)
	if err != nil {
		return nil, fmt.Errorf("error creating Managed Dir with prefix %v: %w", prefix, err)
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	// cleanup any previous runs
	if err := atomic.CleanupTemps(root); err != nil {
		log.Warn("couldn't clean stale temp files", "dir", abs, "err", err)
	}

	return &ManagedDir{abs, mode}, nil
}

func (m *ManagedDir) Install(ctx context.Context, path string, data []byte) error {
	root, err := os.OpenRoot(m.prefix)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return atomic.Write(root, path, m.mode, data)
}

func (m *ManagedDir) Rename(ctx context.Context, path, newPath string) error {
	root, err := os.OpenRoot(m.prefix)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return root.Rename(path, newPath)
}

// List(ctx context.Context,path,prefix string) where path is the subpath to start at
// and prefix is a what the filenames need to start with
func (m *ManagedDir) List(ctx context.Context, subpath, prefix string) ([]string, error) {
	root, err := os.OpenRoot(m.prefix)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	rfs := root.FS()
	rootpath := subpath
	if rootpath == "" {
		rootpath = "."
	}
	var result []string
	err = fs.WalkDir(rfs, rootpath, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(e.Name(), prefix) {
			result = append(result, e.Name())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *ManagedDir) Exists(ctx context.Context, path string) (bool, error) {
	root, err := os.OpenRoot(m.prefix)
	if err != nil {
		return false, err
	}
	defer func() { _ = root.Close() }()
	_, err = root.Stat(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	return err == nil, nil
}

func (m *ManagedDir) Remove(ctx context.Context, path string) error {
	root, err := os.OpenRoot(m.prefix)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// nothing to do
			return nil
		}
		return err
	}
	defer func() { _ = root.Close() }()
	err = root.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (m *ManagedDir) Get(ctx context.Context, path string) ([]byte, error) {
	root, err := os.OpenRoot(m.prefix)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return root.ReadFile(path)
}
