package sops

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
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

func (s *SopsStore) Lookup(_ context.Context, f attributes.AttributesFilter) map[string]any {
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
		decrypted, err := decrypt.File(v, filepath.Ext(v))
		if err != nil {
			log.Fatalf("error decrypting SOPS file %v: %v", v, err)
		}
		if formats.IsYAMLFile(v) {
			err = yaml.Unmarshal(decrypted, &attrs)
			if err != nil {
				log.Fatal(err)
			}
		} else if formats.IsJSONFile(v) {
			err = json.Unmarshal(decrypted, &attrs)
			if err != nil {
				log.Fatal(err)
			}
		} else if formats.IsIniFile(v) {
			// TODO this probably doesn't work?
			interformat, err := ini.Load(decrypted)
			if err != nil {
				log.Fatal(err)
			}
			err = interformat.MapTo(attrs)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatalf("invalid sops file: %v", v)
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
	return results
}
