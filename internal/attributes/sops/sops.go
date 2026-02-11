package sops

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/decrypt"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v3"
	"primamateria.systems/materia/internal/attributes"
)

type SopsStore struct {
	vaultfiles    []string
	generalVaults []string
	loadAllVaults bool
}

func NewSopsStore(c Config, sourceDir string) (*SopsStore, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	var s SopsStore
	s.generalVaults = c.GeneralVaults
	s.loadAllVaults = c.LoadAllVaults
	err := filepath.WalkDir(filepath.Join(sourceDir, c.BaseDir), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" {
			return nil
		}
		// we're not supporting binary or env files here
		// note, the formats package isn't technically stable
		if formats.IsIniFile(path) || formats.IsJSONFile(path) || formats.IsYAMLFile(path) {
			if c.Suffix != "" {
				if strings.Contains(path, c.Suffix) {
					s.vaultfiles = append(s.vaultfiles, path)
				}
			} else {
				s.vaultfiles = append(s.vaultfiles, path)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func (s *SopsStore) Lookup(ctx context.Context, f attributes.AttributesFilter) (map[string]any, error) {
	attrs := attributes.AttributeVault{}

	results := make(map[string]any)
	var files []string
	if s.loadAllVaults {
		files = s.vaultfiles
	} else {
		hostFiles := make([]string, 0, len(s.vaultfiles))
		roleFiles := make([]string, 0, len(s.vaultfiles))
		generalFiles := make([]string, 0, len(s.vaultfiles))
		for _, v := range s.vaultfiles {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
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
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		decrypted, err := decrypt.File(v, filepath.Ext(v))
		if err != nil {
			return nil, fmt.Errorf("error decrypting SOPS file %v: %v", v, err)
		}
		if formats.IsYAMLFile(v) {
			err = yaml.Unmarshal(decrypted, &attrs)
			if err != nil {
				return nil, fmt.Errorf("error unmarshaling SOPS YAML %v: %v", v, err)
			}
		} else if formats.IsJSONFile(v) {
			err = json.Unmarshal(decrypted, &attrs)
			if err != nil {
				return nil, fmt.Errorf("error unmarshaling SOPS JSON %v: %v", v, err)
			}
		} else if formats.IsIniFile(v) {
			// TODO this probably doesn't work?
			interformat, err := ini.Load(decrypted)
			if err != nil {
				return nil, fmt.Errorf("error unmarshaling SOPS INI %v: %v", v, err)
			}
			err = interformat.MapTo(attrs)
			if err != nil {
				return nil, fmt.Errorf("error mapping SOPS INI %v: %v", v, err)
			}
		} else {
			return nil, fmt.Errorf("invalid sops file: %v", v)
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
