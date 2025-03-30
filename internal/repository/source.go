package repository

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
)

type SourceComponentRepository struct {
	DataPrefix string
}

func (c SourceComponentRepository) Validate() error {
	if c.DataPrefix == "" {
		return errors.New("no data prefix")
	}
	return nil
}

func (c *SourceComponentRepository) Install(ctx context.Context, path string, _ *bytes.Buffer) error {
	return errors.New("can't install a source component")
}

func (c *SourceComponentRepository) Remove(ctx context.Context, path string) error {
	return errors.New("can't remove a source component")
}

func (c *SourceComponentRepository) Purge(ctx context.Context, path string) error {
	return errors.New("can't purge a source component")
}

func (c *SourceComponentRepository) Exists(ctx context.Context, path string) (bool, error) {
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

func (c *SourceComponentRepository) Get(ctx context.Context, path string) (string, error) {
	panic("not implemented") // TODO: Implement
}

func (c *SourceComponentRepository) List(ctx context.Context) ([]string, error) {
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

func (c *SourceComponentRepository) ListResources(ctx context.Context, name string) ([]string, error) {
	var results []string
	if err := c.Validate(); err != nil {
		return results, err
	}
	if name == "" {
		return results, errors.New("no name specified")
	}
	dataPath := filepath.Join(c.DataPrefix, name)
	resources, err := os.ReadDir(dataPath)
	if err != nil {
		return results, err
	}
	for _, r := range resources {
		results = append(results, filepath.Join(dataPath, r.Name()))
	}

	return results, nil
}

func (c *SourceComponentRepository) Clean(ctx context.Context) error {
	return errors.New("can't clean source repository")
}
