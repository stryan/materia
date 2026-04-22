package manifests

import (
	"errors"
	"slices"

	"github.com/BurntSushi/toml"
	filesource "primamateria.systems/materia/internal/source/file"
	"primamateria.systems/materia/internal/source/git"
	"primamateria.systems/materia/internal/source/oci"
)

var (
	MateriaManifestFile           = "MANIFEST.toml"
	ErrHostNotInManifest          = errors.New("host not in manifest")
	ErrComponentNotAssignedToHost = errors.New("component not assigned to host")
)

type SnippetConfig struct {
	Name       string   `toml:"Name"`
	Body       string   `toml:"Body"`
	Parameters []string `toml:"Parameters"`
}

type RemoteComponentConfig struct {
	GitSource  *git.Config        `toml:"git,omitempty"`
	OciSource  *oci.Config        `toml:"oci,omitempty"`
	FileSource *filesource.Config `toml:"file,omitempty"`
	Subpath    string             `toml:"subpath"`
}

type MateriaManifest struct {
	Hosts       map[string]Host                  `toml:"Hosts" koanf:"Hosts"`
	Snippets    []SnippetConfig                  `toml:"Snippets" koanf:"Snippets"`
	Roles       map[string]Role                  `toml:"Roles" koanf:"Roles"`
	RoleCommand string                           `toml:"RoleCommand" koanf:"RoleCommand"`
	Remotes     map[string]RemoteComponentConfig `toml:"Remotes" koanf:"Remotes"`
}

type Host struct {
	Components []string                     `toml:"Components"`
	Roles      []string                     `toml:"Roles"`
	Overrides  map[string]ComponentManifest `toml:"Overrides"`
	Extensions map[string]ComponentManifest `toml:"Extensions"`
}

type Role struct {
	Components []string `toml:"Components"`
}

func LoadMateriaManifest(path string) (*MateriaManifest, error) {
	var m MateriaManifest
	_, err := toml.DecodeFile(path, &m)
	if err != nil {
		return nil, err
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

func (m *MateriaManifest) GetComponentOverride(hostname, componentName string) (*ComponentManifest, error) {
	hostConfig, ok := m.Hosts[hostname]
	if !ok {
		return nil, ErrHostNotInManifest
	}

	if o, ok := hostConfig.Overrides[componentName]; ok {
		return &o, nil
	}
	return nil, ErrComponentNotAssignedToHost
}

func (m *MateriaManifest) GetComponentExtension(hostname, componentName string) (*ComponentManifest, error) {
	hostConfig, ok := m.Hosts[hostname]
	if !ok {
		return nil, ErrHostNotInManifest
	}

	if o, ok := hostConfig.Extensions[componentName]; ok {
		return &o, nil
	}
	return nil, ErrComponentNotAssignedToHost
}
