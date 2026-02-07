package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type FileRepository struct {
	Prefix string
}

func NewFileRepository(prefix string) (*FileRepository, error) {
	if _, err := os.Stat(prefix); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.Mkdir(prefix, 0o755)
			if err != nil {
				return nil, fmt.Errorf("error creating FileRepository with prefix %v: %w", prefix, err)
			}
		}
	}
	return &FileRepository{prefix}, nil
}

func (filerepository *FileRepository) Install(ctx context.Context, path string, data []byte) error {
	err := os.WriteFile(filepath.Join(filerepository.Prefix, path), data, 0o755)
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

func (filerepository *FileRepository) Get(ctx context.Context, path string) (string, error) {
	return filepath.Join(filerepository.Prefix, path), nil
}

func (filerepository *FileRepository) List(ctx context.Context) ([]string, error) {
	panic("unimplemented")
}

func (filerepository *FileRepository) Clean(ctx context.Context) error {
	entries, err := os.ReadDir(filerepository.Prefix)
	if err != nil {
		return err
	}
	for _, v := range entries {
		err := os.RemoveAll(filepath.Join(filerepository.Prefix, v.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}
