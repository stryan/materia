package manifests

import (
	"errors"
	"path/filepath"
	"slices"

	"git.saintnet.tech/stryan/materia/internal/secrets"
	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	"git.saintnet.tech/stryan/materia/internal/secrets/mem"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type SnippetConfig struct {
	Name, Body string
	Parameters []string
}

type MateriaManifest struct {
	Secrets       string
	SecretsConfig secrets.SecretsConfig
	Hosts         map[string]Host
	Snippets      []SnippetConfig
	Roles         map[string]Role
	RoleCommand   string
}

type Host struct {
	Components []string
	Roles      []string
}

type Role struct {
	Components []string
}

func LoadMateriaManifest(path string) (*MateriaManifest, error) {
	k := koanf.New(".")
	err := k.Load(file.Provider(path), toml.Parser())
	if err != nil {
		return nil, err
	}
	var m MateriaManifest
	err = k.Unmarshal("", &m)
	if err != nil {
		return nil, err
	}
	switch m.Secrets {
	case "age":
		m.SecretsConfig = age.Config{
			IdentPath: k.MustString("age.idents"),
			RepoPath:  filepath.Dir(path),
		}
	case "memory":
		m.SecretsConfig = mem.MemoryConfig{}
	}
	return &m, nil
}

func (m MateriaManifest) Validate() error {
	for k, v := range m.Hosts {
		if k == "" {
			return errors.New("materia manifest can't have empty host name")
		}
		if slices.Contains(v.Components, "") {
			return errors.New("materia manifest can't have empty component name")
		}
		if slices.Contains(v.Roles, "") {
			return errors.New("materia manifest can't have empty role name")
		}
	}
	return nil
}
