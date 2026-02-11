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

func (s *FileStore) Lookup(_ context.Context, f attributes.AttributesFilter) map[string]any {
	secrets := attributes.AttributeVault{}

	results := make(map[string]any)
	var files []string
	if s.loadAllVaults {
		files = s.vaultfiles
	} else {
		hostFiles := make([]string, 0, len(s.vaultfiles))
		roleFiles := make([]string, 0, len(s.vaultfiles))
		generalFiles := make([]string, 0, len(s.vaultfiles))
		for _, v := range s.vaultfiles {
			if slices.Contains(s.generalVaults, filepath.Base(v)) {
				generalFiles = append(generalFiles, v)
			}
			if strings.Contains(v, f.Hostname) {
				hostFiles = append(hostFiles, v)
			}
			for _, r := range f.Roles {
				if strings.Contains(v, r) {
					roleFiles = append(roleFiles, v)
				}
			}
		}
		// file list is in order of General Vaults, Role Vaults, Host Vaults
		// So host file keys override role keys override general keys
		files = append(generalFiles, roleFiles...)
		files = append(files, hostFiles...)
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
		if len(f.Roles) != 0 {
			for _, r := range f.Roles {
				maps.Copy(results, secrets.Roles[r])
			}
		}

		if f.Component != "" {
			maps.Copy(results, secrets.Components[f.Component])
		}
		if f.Hostname != "" {
			maps.Copy(results, secrets.Hosts[f.Hostname])
		}

	}
	return results
}
