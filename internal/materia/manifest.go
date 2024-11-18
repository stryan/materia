package materia

import (
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
}

type Host struct {
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
	return nil
}

type ComponentManifest struct {
	Services []string
	Defaults map[string]interface{}
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
