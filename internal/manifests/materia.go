package manifests

import (
	"errors"
	"slices"

	"github.com/charmbracelet/log"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var MateriaManifestFile = "MANIFEST.toml"

type SnippetConfig struct {
	Name, Body string
	Parameters []string
}

type MateriaManifest struct {
	Secrets     string
	Hosts       map[string]Host
	Snippets    []SnippetConfig
	Roles       map[string]Role
	RoleCommand string
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
	if m.Secrets != "" {
		log.Warn("WARNING: configuring secrets in the manifest file is deprecated and will be removed next release")
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
