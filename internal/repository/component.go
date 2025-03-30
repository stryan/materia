package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type ComponentRepository interface {
	Install(ctx context.Context, path string, data *bytes.Buffer) error
	Remove(ctx context.Context, path string) error
	Exists(ctx context.Context, path string) (bool, error)
	Get(ctx context.Context, path string) (string, error)
	List(ctx context.Context) ([]string, error)
	Clean(ctx context.Context) error
}

type HostComponentRepository struct {
	DataPrefix    string
	QuadletPrefix string
}

func (c HostComponentRepository) Validate() error {
	if c.DataPrefix == "" {
		return errors.New("no data prefix")
	}
	if c.QuadletPrefix == "" {
		return errors.New("no quadlet prefix")
	}
	return nil
}

func (c *HostComponentRepository) Install(ctx context.Context, path string, _ *bytes.Buffer) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if path == "" {
		return errors.New("no path specified")
	}
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

func (c *HostComponentRepository) Remove(ctx context.Context, path string) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if path == "" {
		return errors.New("no path specified")
	}
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

func (c *HostComponentRepository) Purge(ctx context.Context, path string) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if path == "" {
		return errors.New("no path specified")
	}
	err := os.RemoveAll(filepath.Join(c.DataPrefix, path))
	if err != nil {
		return err
	}

	err = os.RemoveAll(filepath.Join(c.QuadletPrefix, path))
	return err
}

func (c *HostComponentRepository) Exists(ctx context.Context, path string) (bool, error) {
	if err := c.Validate(); err != nil {
		return false, err
	}
	if path == "" {
		return false, errors.New("no path specified")
	}
	_, err := os.Stat(filepath.Join(c.DataPrefix, path))
	if err != nil {
		return false, err
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

func (c *HostComponentRepository) Get(ctx context.Context, path string) (string, error) {
	panic("not implemented") // TODO: Implement
}

func (c *HostComponentRepository) List(ctx context.Context) ([]string, error) {
	var compPaths []string
	if err := c.Validate(); err != nil {
		return compPaths, err
	}
	entries, err := os.ReadDir(c.DataPrefix)
	if err != nil {
		return nil, err
	}
	for _, v := range entries {
		if v.IsDir() {
			compPaths = append(compPaths, filepath.Join(c.DataPrefix, v.Name()))
		}
	}
	return compPaths, nil
}

func (c *HostComponentRepository) ListResources(ctx context.Context, name string) ([]string, error) {
	var results []string
	if err := c.Validate(); err != nil {
		return results, err
	}
	if name == "" {
		return results, errors.New("no name specified")
	}
	quadletsPath := filepath.Join(c.QuadletPrefix, name)
	dataPath := filepath.Join(c.DataPrefix, name)
	quadlets, err := os.ReadDir(quadletsPath)
	if err != nil {
		return results, err
	}
	for _, q := range quadlets {
		if q.Name() == ".materia_managed" {
			continue
		}
		results = append(results, filepath.Join(quadletsPath, q.Name()))
	}
	resources, err := os.ReadDir(dataPath)
	if err != nil {
		return results, err
	}
	for _, r := range resources {
		results = append(results, filepath.Join(dataPath, r.Name()))
	}

	return results, nil
}

func (c *HostComponentRepository) Clean(ctx context.Context) error {
	if err := c.Validate(); err != nil {
		return err
	}
	entries, err := os.ReadDir(c.QuadletPrefix)
	if err != nil {
		return err
	}
	for _, v := range entries {
		_, err := os.Stat(fmt.Sprintf("%v/%v/.materia_managed", c.QuadletPrefix, v.Name()))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		err = os.RemoveAll(filepath.Join(c.QuadletPrefix, v.Name()))
		if err != nil {
			return err
		}

	}
	return nil
}
