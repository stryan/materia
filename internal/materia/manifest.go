package materia

import (
	"errors"
	"path/filepath"

	"git.saintnet.tech/stryan/materia/internal/secrets"
	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	"git.saintnet.tech/stryan/materia/internal/secrets/mem"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

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
		for _, c := range v.Components {
			if c == "" {
				return errors.New("materia manifest can't have empty component name")
			}
		}
		for _, r := range v.Roles {
			if r == "" {
				return errors.New("materia manifest can't have empty role name")
			}
		}
	}
	return nil
}

type VolumeResourceConfig struct {
	Volume      string
	Resource    string
	Path        string
	Owner, Mode string
}

func (vrc VolumeResourceConfig) Validate() error {
	if vrc.Volume == "" {
		return errors.New("need volume")
	}
	if vrc.Resource == "" {
		return errors.New("need resource")
	}
	if vrc.Path == "" {
		return errors.New("need in-volume path")
	}
	return nil
}

type ComponentManifest struct {
	Services        []string
	Enabled         []string
	NoServices      bool
	Defaults        map[string]interface{}
	Snippets        []SnippetConfig
	VolumeResources map[string]VolumeResourceConfig
	Scripts         []string
}

func LoadComponentManifest(path string) (*ComponentManifest, error) {
	k := koanf.New(".")
	err := k.Load(file.Provider(path), toml.Parser())
	if err != nil {
		return nil, err
	}
	var c ComponentManifest
	err = k.Unmarshal("", &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
