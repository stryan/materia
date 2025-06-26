package file

import (
	"bytes"
	"context"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"primamateria.systems/materia/internal/secrets"
	"github.com/BurntSushi/toml"
)

type FileStore struct {
	vaultfiles    []string
	generalVaults []string
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
		c.GeneralVaults = []string{"vault.toml", "secrets.toml"}
	}
	f.generalVaults = c.GeneralVaults
	return &f, nil
}

func (s *FileStore) Lookup(_ context.Context, f secrets.SecretFilter) map[string]any {
	secrets := secrets.SecretsVault{}

	results := make(map[string]any)
	files := []string{}
	for _, v := range s.vaultfiles {
		if strings.Contains(v, f.Hostname) || slices.Contains(s.generalVaults, filepath.Base(v)) {
			files = append(files, v)
		}
		for _, r := range f.Roles {
			if strings.Contains(v, r) {
				files = append(files, v)
			}
		}
	}
	for _, v := range files {
		file, err := os.Open(v)
		if err != nil {
			log.Fatal(err)
		}
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(file)
		if err != nil {
			log.Fatal(err)
		}
		err = toml.Unmarshal(buf.Bytes(), &secrets)
		if err != nil {
			log.Fatal(err)
		}

		maps.Copy(results, secrets.Globals)
		if f.Component != "" {
			maps.Copy(results, secrets.Components[f.Component])
		}
		if f.Hostname != "" {
			maps.Copy(results, secrets.Hosts[f.Hostname])
		}
		if len(f.Roles) != 0 {
			for _, r := range f.Roles {
				maps.Copy(results, secrets.Roles[r])
			}
		}

	}
	return results
}
