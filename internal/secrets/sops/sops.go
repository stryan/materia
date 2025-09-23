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
	"primamateria.systems/materia/internal/secrets"
)

type SopsStore struct {
	vaultfiles    []string
	generalVaults []string
}

func NewSopsStore(c Config, sourceDir string) (*SopsStore, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	var s SopsStore
	s.generalVaults = c.GeneralVaults
	err := filepath.WalkDir(filepath.Join(sourceDir, c.BaseDir), func(path string, d fs.DirEntry, err error) error {
		if d.Name() == ".git" {
			return nil
		}
		// we're not supporting binary or env files here
		// note, the formats package isn't technically stable
		if formats.IsIniFile(path) || formats.IsJSONFile(path) || formats.IsYAMLFile(path) {
			s.vaultfiles = append(s.vaultfiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func (s *SopsStore) Lookup(_ context.Context, f secrets.SecretFilter) map[string]any {
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
		decrypted, err := decrypt.File(v, filepath.Ext(v))
		if err != nil {
			log.Fatalf("error decrypting SOPS file: %v", err)
		}
		if formats.IsYAMLFile(v) {
			err = yaml.Unmarshal(decrypted, &secrets)
			if err != nil {
				log.Fatal(err)
			}
		} else if formats.IsJSONFile(v) {
			err = json.Unmarshal(decrypted, &secrets)
			if err != nil {
				log.Fatal(err)
			}
		} else if formats.IsIniFile(v) {
			// TODO this probably doesn't work?
			interformat, err := ini.Load(decrypted)
			if err != nil {
				log.Fatal(err)
			}
			err = interformat.MapTo(secrets)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatalf("invalid sops file: %v", v)
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
