package age

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"filippo.io/age"
	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
	"primamateria.systems/materia/internal/attributes"
)

type AgeStore struct {
	identities    []age.Identity
	vaultfiles    []string
	generalVaults []string
}

func NewAgeStore(c Config, sourceDir string) (*AgeStore, error) {
	err := c.Validate()
	if err != nil {
		return nil, err
	}
	var a AgeStore
	dir := filepath.Dir(c.IdentPath)
	// TODO this was added for testing, is it needed?
	if dir == "." {
		c.IdentPath = filepath.Join(sourceDir, c.IdentPath)
	}
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

func (a *AgeStore) Lookup(_ context.Context, f attributes.AttributesFilter) map[string]any {
	attrs := attributes.AttributeVault{}

	results := make(map[string]any)
	files := []string{}
	for _, v := range a.vaultfiles {
		if strings.Contains(v, f.Hostname) || slices.Contains(a.generalVaults, filepath.Base(v)) {
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
		decrypted, err := age.Decrypt(file, a.identities...)
		if err != nil {
			log.Fatal(err)
		}
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(decrypted)
		if err != nil {
			log.Fatal(err)
		}
		err = toml.Unmarshal(buf.Bytes(), &attrs)
		if err != nil {
			log.Fatal(err)
		}

		maps.Copy(results, attrs.Globals)
		if f.Component != "" {
			maps.Copy(results, attrs.Components[f.Component])
		}
		if f.Hostname != "" {
			maps.Copy(results, attrs.Hosts[f.Hostname])
		}
		if len(f.Roles) != 0 {
			for _, r := range f.Roles {
				maps.Copy(results, attrs.Roles[r])
			}
		}

	}
	return results
}
