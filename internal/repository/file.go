package repository

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
)

type FileRepository struct {
	Prefix string
}

func (filerepository *FileRepository) Install(ctx context.Context, path string, data *bytes.Buffer) error {
	err := os.WriteFile(filepath.Join(filerepository.Prefix, path), data.Bytes(), 0o755)
	if err != nil {
		return err
	}
	return nil
}

func (filerepository *FileRepository) Remove(ctx context.Context, path string) error {
	err := os.Remove(filepath.Join(filerepository.Prefix, path))
	if err != nil {
		return err
	}
	return nil
}

func (filerepository *FileRepository) Exists(ctx context.Context, path string) (bool, error) {
	_, err := os.Stat(filepath.Join(filerepository.Prefix, path))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return true, nil
}
