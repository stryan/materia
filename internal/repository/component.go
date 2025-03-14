package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type ComponentRepository struct {
	DataPrefix    string
	QuadletPrefix string
}

func (c *ComponentRepository) Install(ctx context.Context, path string, _ *bytes.Buffer) error {
	err := os.Mkdir(filepath.Join(c.DataPrefix, path), 0o755)
	if err != nil {
		return fmt.Errorf("error installing component %v: %w", filepath.Join(c.DataPrefix, path), err)
	}
	qpath := filepath.Join(c.QuadletPrefix, path)
	err = os.Mkdir(qpath, 0o755)
	if err != nil {
		return fmt.Errorf("error installing component: %w", err)
	}

	qFile, err := os.OpenFile(fmt.Sprintf("%v/.materia_managed", qpath), os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("error installing component: %w", err)
	}
	defer qFile.Close()
	return nil
}

func (c *ComponentRepository) Remove(ctx context.Context, path string) error {
	entries, err := os.ReadDir(filepath.Join(c.DataPrefix, path))
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("component data folder not empty")
	}
	entries, err = os.ReadDir(filepath.Join(c.DataPrefix, path))
	if err != nil {
		return err
	}
	if len(entries) != 1 {
		return errors.New("component quadlet folder not empty")
	}
	err = os.Remove(filepath.Join(c.DataPrefix, path))
	if err != nil {
		return err
	}

	err = os.Remove(filepath.Join(c.QuadletPrefix, path, ".materia_managed"))
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(c.QuadletPrefix, path))
	return err
}

func (componentrepository *ComponentRepository) Exists(ctx context.Context, path string) (bool, error) {
	panic("not implemented") // TODO: Implement
}
