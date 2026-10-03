package repository

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"charm.land/log/v2"
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
	if err := cleanupAtomicTemps(root); err != nil {
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
	return atomicWrite(root, path, m.mode, data)
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
