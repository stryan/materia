package age

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"git.saintnet.tech/stryan/materia/internal/secrets"
	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
)

type AgeStore struct {
	identities []age.Identity
	vaultfiles []string
}

type Config struct {
	IdentPath string
	RepoPath  string
}

func (c *Config) Validate() error {
	if c.RepoPath == "" {
		return errors.New("invalid repo path for age")
	}
	if c.IdentPath == "" {
		return errors.New("invalid identities location for age")
	}
	return nil
}

func NewAgeStore(c Config) (*AgeStore, error) {
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
	err = filepath.WalkDir(c.RepoPath, func(path string, d fs.DirEntry, err error) error {
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

func (a *AgeStore) All(_ context.Context) map[string]interface{} {
	results := make(map[string]interface{})
	for _, v := range a.vaultfiles {
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
		err = toml.Unmarshal(buf.Bytes(), &results)
		if err != nil {
			log.Fatal(err)
		}
	}
	return results
}

func (a *AgeStore) Lookup(_ context.Context, f secrets.SecretFilter) map[string]interface{} {
	results := make(map[string]interface{})
	files := []string{}
	for _, v := range a.vaultfiles {
		if strings.Contains(v, f.Hostname) || strings.Contains(v, f.Role) ||
			filepath.Base(v) == "vault.age" || filepath.Base(v) == "secrets.age" {
			files = append(files, v)
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
		err = toml.Unmarshal(buf.Bytes(), &results)
		if err != nil {
			log.Fatal(err)
		}
	}
	return results
}
