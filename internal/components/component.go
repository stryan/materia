package components

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"primamateria.systems/materia/pkg/manifests"
)

const DefaultComponentVersion = 1

var ErrCorruptComponent = errors.New("error corrupt component")

type Component struct {
	Name             string
	Settings         manifests.Settings
	Resources        *ResourceSet
	Scripted         bool
	State            ComponentLifecycle
	Defaults         map[string]any
	ServiceResources map[string]manifests.ServiceResourceConfig
	Version          int
}

//go:generate stringer -type ComponentLifecycle -trimprefix State
type ComponentLifecycle int

const (
	StateUnknown ComponentLifecycle = iota
	StateStale
	StateFresh
	StateOK
	StateMayNeedUpdate
	StateNeedUpdate
	StateNeedRemoval
	StateRemoved
)

type ComponentVersion struct {
	Version int
}

func NewComponent(name string) *Component {
	return &Component{
		Name:             name,
		State:            StateStale,
		Defaults:         make(map[string]any),
		ServiceResources: make(map[string]manifests.ServiceResourceConfig),
		Resources:        NewResourceSet(),
	}
}

func (c *Component) ApplyManifest(man *manifests.ComponentManifest) error {
	maps.Copy(c.Defaults, man.Defaults)
	c.Settings = man.Settings
	slices.Sort(man.Secrets)
	var secretResources []Resource
	for _, s := range man.Secrets {
		secretResources = append(secretResources, Resource{
			Path:     s,
			Kind:     ResourceTypePodmanSecret,
			Parent:   c.Name,
			Template: false,
		})
	}
	for _, s := range man.Services {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("invalid service for component: %w", err)
		}
		c.ServiceResources[s.Service] = s
	}
	for _, r := range c.Resources.List() {
		if r.Kind != ResourceTypeScript && slices.Contains(man.Scripts, r.Path) {
			r.Kind = ResourceTypeScript
			if err := c.Resources.Add(r); err != nil {
				return err
			}
		}
	}
	for _, s := range secretResources {
		c.Resources.Set(s)
	}
	return nil
}

func (c *Component) String() string {
	return fmt.Sprintf("{c %v %v Rs: %v Ss: %v D: [%v]}", c.Name, c.State, c.Resources.Size(), len(c.ServiceResources), c.Defaults)
}

func (c Component) Validate() error {
	if c.Name == "" {
		return errors.New("component without name")
	}
	if c.State == StateUnknown {
		return errors.New("component with unknown state")
	}
	return nil
}

func (c *Component) VersonData() (*bytes.Buffer, error) {
	vd := make(map[string]any)
	vd["Version"] = c.Version
	buffer, err := toml.Parser().Marshal(vd)
	if err != nil {
		return nil, errors.New("can't create version data")
	}
	return bytes.NewBuffer(buffer), nil
}

func FindResourceType(file string) ResourceType {
	if filepath.Base(file) == "setup.sh" || filepath.Base(file) == "cleanup.sh" {
		return ResourceTypeComponentScript
	}
	filename := strings.TrimSuffix(file, ".gotmpl")
	switch filepath.Ext(filename) {
	case ".pod":
		return ResourceTypePod
	case ".container":
		return ResourceTypeContainer
	case ".network":
		return ResourceTypeNetwork
	case ".volume":
		return ResourceTypeVolume
	case ".build":
		return ResourceTypeBuild
	case ".image":
		return ResourceTypeImage
	case ".kube":
		return ResourceTypeKube
	case ".toml":
		if filepath.Base(file) == manifests.ComponentManifestFile {
			return ResourceTypeManifest
		}
		return ResourceTypeFile
	case ".service", ".timer", ".target", ".socket", ".path", ".mount", ".automount", ".swap", ".slice", ".scope", ".device": // IDEA parse from man page?
		return ResourceTypeService
	case ".sh":
		return ResourceTypeScript
	default:
		if len(file) == 0 {
			return ResourceTypeHost
		}
		if file[len(file)-1:] == "/" {
			return ResourceTypeDirectory
		}
		return ResourceTypeFile

	}
}

func IsTemplate(file string) bool {
	return strings.HasSuffix(file, ".gotmpl")
}
