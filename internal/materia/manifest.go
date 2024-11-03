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

type Manifest struct {
	Secrets       string
	SecretsConfig secrets.SecretsConfig
	Hosts         map[string]Host
}

type Host struct {
	Components []string
	Order      []string
}

func LoadManifest(path string) (*Manifest, error) {
	k := koanf.New(".")
	err := k.Load(file.Provider(path), toml.Parser())
	if err != nil {
		return nil, err
	}
	var m Manifest
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

func (m Manifest) Validate() error {
	return nil
}
