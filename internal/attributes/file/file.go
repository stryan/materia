package file

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"primamateria.systems/materia/internal/attributes"
)

type FileStore struct {
	vaultfiles    []string
	generalVaults []string
	loadAllVaults bool
}

func NewFileStore(c Config, sourceDir string) (*FileStore, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	var f FileStore

	err := filepath.WalkDir(filepath.Join(sourceDir, c.BaseDir), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" || d.Name() == "MANIFEST.toml" {
			return nil
		}
		if filepath.Ext(path) == ".toml" {
			f.vaultfiles = append(f.vaultfiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(c.GeneralVaults) == 0 {
		c.GeneralVaults = []string{"vault.toml", "attributes.toml"}
	}
	f.generalVaults = c.GeneralVaults
	f.loadAllVaults = c.LoadAllVaults
	return &f, nil
}

func (s *FileStore) Lookup(ctx context.Context, f attributes.AttributesFilter) (map[string]any, error) {
	attrs := attributes.AttributeVault{}

	results := make(map[string]any)
	var files []string
	var err error
	if s.loadAllVaults {
		files = s.vaultfiles
	} else {
		files, err = attributes.SortedVaultFiles(ctx, f, s.vaultfiles, s.generalVaults)
		if err != nil {
			return nil, fmt.Errorf("can't prepare vault file list: %w", err)
		}
	}

	for _, v := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		file, err := os.Open(v)
		if err != nil {
			return nil, err
		}
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(file)
		if err != nil {
			return nil, err
		}
		err = toml.Unmarshal(buf.Bytes(), &attrs)
		if err != nil {
			return nil, err
		}
		attributes.ExtractVaultAttributes(results, attrs, f)

	}
	return results, nil
}
