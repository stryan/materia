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

var (
	ErrCorruptComponent = errors.New("error corrupt component")
	ErrUnloadedManifest = errors.New("error unloaded manifest")
)

type Component struct {
	Name          string
	Settings      manifests.Settings
	Resources     *ResourceSet
	State         ComponentLifecycle
	Defaults      map[string]any
	Services      *ServiceSet
	Version       int
	SetupScript   string
	CleanupScript string
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
	StateRoot
)

type ComponentVersion struct {
	Version int
}

func NewComponent(name string) *Component {
	return &Component{
		Name:      name,
		State:     StateStale,
		Defaults:  make(map[string]any),
		Services:  NewServiceSet(),
		Resources: NewResourceSet(),
	}
}

func NewRootComponent() *Component {
	return &Component{
		Name:      "root",
		State:     StateRoot,
		Defaults:  make(map[string]any),
		Services:  NewServiceSet(),
		Resources: NewResourceSet(),
	}
}

func (c *Component) GetManifest() (*manifests.ComponentManifest, error) {
	if c.Resources == nil {
		return nil, errors.New("unloaded component")
	}
	manResource, err := c.Resources.Get(manifests.ComponentManifestFile)
	if err != nil {
		return nil, err
	}
	if manResource.Content == "" {
		return nil, ErrUnloadedManifest
	}
	return manifests.LoadComponentManifestFromContent([]byte(manResource.Content))
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
		c.Services.Add(s)
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
	c.SetupScript = man.Settings.SetupScript
	c.CleanupScript = man.Settings.CleanupScript
	return nil
}

func (c *Component) String() string {
	numRes := -1
	if c.Resources != nil {
		numRes = c.Resources.Size()
	}
	numServes := -1
	if c.Services != nil {
		numServes = c.Services.Size()
	}
	return fmt.Sprintf("{c %v %v Rs: %v Ss: %v D: [%v]}", c.Name, c.State, numRes, numServes, c.Defaults)
}

func (c Component) Validate() error {
	if c.Name == "" {
		return errors.New("component without name")
	}
	if c.State == StateUnknown {
		return errors.New("component with unknown state")
	}
	if c.Resources == nil {
		return errors.New("component without resource set")
	}
	if c.Services == nil {
		return errors.New("component without services set")
	}
	if c.SetupScript != "" && c.CleanupScript == "" {
		return errors.New("component has setup script but no cleanup script")
	}
	if c.SetupScript == "" && c.CleanupScript != "" {
		return errors.New("component has cleanup script but no setup script")
	}
	return nil
}

func (c *Component) VersonData() (*bytes.Buffer, error) {
	vd := make(map[string]any)
	vd["Version"] = c.Version
	buffer, err := toml.Parser().Marshal(vd)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(buffer), nil
}

func (c *Component) ToResource() Resource {
	return Resource{
		Path:   c.Name,
		Parent: c.Name,
		Kind:   ResourceTypeComponent,
	}
}

func (c *Component) ToAppfile() []byte {
	appfiles := []string{}
	for _, r := range c.Resources.List() {
		if r.IsQuadlet() {
			appfiles = append(appfiles, r.Path)
		}
	}
	appfile := strings.Join(appfiles, "\n")
	return []byte(appfile)
}

func FindResourceType(file string) ResourceType {
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
	case ".app":
		return ResourceTypeAppFile
	case ".quadlets":
		return ResourceTypeCombined
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
