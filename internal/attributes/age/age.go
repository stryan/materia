package age

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"filippo.io/age"
	"github.com/BurntSushi/toml"
	"primamateria.systems/materia/internal/attributes"
)

type AgeStore struct {
	identities    []age.Identity
	vaultfiles    []string
	generalVaults []string
	loadAllVaults bool
}

func NewAgeStore(c Config, sourceDir string) (*AgeStore, error) {
	err := c.Validate()
	if err != nil {
		return nil, err
	}
	var a AgeStore
	ifile, err := os.Open(c.IdentPath)
	if err != nil {
		return nil, err
	}
	idents, err := age.ParseIdentities(ifile)
	if err != nil {
		return nil, err
	}
	if len(idents) == 0 {
		return nil, errors.New("need at least one identity")
	}
	a.identities = idents
	if len(c.GeneralVaults) == 0 {
		c.GeneralVaults = []string{"vault.age", "attributes.age"}
	}
	a.generalVaults = c.GeneralVaults
	err = filepath.WalkDir(filepath.Join(sourceDir, c.BaseDir), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" {
			return nil
		}
		if filepath.Ext(path) == ".age" {
			a.vaultfiles = append(a.vaultfiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (a *AgeStore) Lookup(ctx context.Context, f attributes.AttributesFilter) (map[string]any, error) {
	attrs := attributes.AttributeVault{}

	results := make(map[string]any)
	var files []string
	if a.loadAllVaults {
		files = a.vaultfiles
	} else {
		hostFiles := make([]string, 0, len(a.vaultfiles))
		roleFiles := make([]string, 0, len(a.vaultfiles))
		generalFiles := make([]string, 0, len(a.vaultfiles))
		for _, v := range a.vaultfiles {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			if slices.Contains(a.generalVaults, filepath.Base(v)) {
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
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		file, err := os.Open(v)
		if err != nil {
			return nil, err
		}
		decrypted, err := age.Decrypt(file, a.identities...)
		if err != nil {
			return nil, fmt.Errorf("unable to decrypt age file %v: %w", v, err)
		}
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(decrypted)
		if err != nil {
			return nil, err
		}
		err = toml.Unmarshal(buf.Bytes(), &attrs)
		if err != nil {
			return nil, err
		}

		maps.Copy(results, attrs.Globals)
		if len(f.Roles) != 0 {
			for _, r := range f.Roles {
				maps.Copy(results, attrs.Roles[r])
			}
		}
		if f.Component != "" {
			maps.Copy(results, attrs.Components[f.Component])
		}
		if f.Hostname != "" {
			maps.Copy(results, attrs.Hosts[f.Hostname])
		}

	}
	return results, nil
}
