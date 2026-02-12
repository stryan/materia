package sops

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
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
		attributes.ExtractVaultAttributes(results, attrs, f)
	}
	return results, nil
}
