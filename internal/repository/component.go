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

func (c *ComponentRepository) Get(ctx context.Context, path string) (string, error) {
	panic("not implemented") // TODO: Implement
}

func (c *ComponentRepository) List(ctx context.Context) ([]string, error) {
	var compPaths []string
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

func (c *ComponentRepository) ListResources(ctx context.Context, name string) ([]string, error) {
	var results []string
	quadletsPath := filepath.Join(c.QuadletPrefix, name)
	fmt.Fprintf(os.Stderr, "FBLTHP[130]: component.go:88: quadletsPath=%+v\n", quadletsPath)
	dataPath := filepath.Join(c.DataPrefix, name)
	fmt.Fprintf(os.Stderr, "FBLTHP[131]: component.go:90: dataPath=%+v\n", dataPath)
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

func (c *ComponentRepository) Clean(ctx context.Context) error {
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
