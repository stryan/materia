package components

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/services"
)

const DefaultComponentVersion = 1

var (
	ErrCorruptComponent = errors.New("error corrupt component")
	ErrUnloadedManifest = errors.New("error unloaded manifest")
	dropInDirRegex      = regexp.MustCompile(`^([a-zA-Z0-9_][a-zA-Z0-9_-]*-?\.)?[a-z]+\.d$`)
)

type Component struct {
	Name           string
	Instance       string
	Overrides      []string
	Settings       manifests.Settings
	Resources      *ResourceSet
	State          ComponentLifecycle
	Defaults       map[string]any
	ServiceConfigs *ServiceConfigSet
	Version        int
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
	instance := ""
	if strings.Contains(name, "@") {
		split := strings.Split(name, "@")
		name = split[0]
		instance = split[1]
	}
	return &Component{
		Name:           name,
		Instance:       instance,
		State:          StateStale,
		Defaults:       make(map[string]any),
		ServiceConfigs: NewServiceConfigSet(),
		Resources:      NewResourceSet(),
	}
}

func NewRootComponent() *Component {
	return &Component{
		Name:           "root",
		State:          StateRoot,
		Defaults:       make(map[string]any),
		ServiceConfigs: NewServiceConfigSet(),
		Resources:      NewResourceSet(),
	}
}

func (c *Component) IsInstanced() bool {
	return c.Instance != ""
}

func (c *Component) Instantiate(template string) string {
	if c.Instance != "" && strings.Contains(template, "@.") {
		return strings.ReplaceAll(template, "@", fmt.Sprintf("@%v", c.Instance))
	}
	return template
}

func (c *Component) InstantiateResource(template Resource) Resource {
	if c.Instance == "" {
		return template
	}
	template.Parent = c.InstanceName()
	if strings.Contains(template.Path, "@.") {
		template.Path = strings.ReplaceAll(template.Path, "@", fmt.Sprintf("@%v", c.Instance))
	}
	return template
}

func (c *Component) InstantiateServiceConfig(template manifests.ServiceResourceConfig) manifests.ServiceResourceConfig {
	if c.Instance == "" {
		return template
	}
	if strings.Contains(template.Service, "@.") {
		template.Service = strings.ReplaceAll(template.Service, "@", fmt.Sprintf("@%v", c.Instance))
	}
	return template
}

func (c *Component) InstanceName() string {
	if c.Instance == "" {
		return c.Name
	}
	return fmt.Sprintf("%v@%v", c.Name, c.Instance)
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
		c.ServiceConfigs.Add(c.InstantiateServiceConfig(s))
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
	numRes := -1
	if c.Resources != nil {
		numRes = c.Resources.Size()
	}
	numServes := -1
	if c.ServiceConfigs != nil {
		numServes = c.ServiceConfigs.Size()
	}
	name := c.Name
	if c.Instance != "" {
		name = fmt.Sprintf("%v@%v", name, c.Instance)
	}
	return fmt.Sprintf("{c %v %v Rs: %v Ss: %v D: [%v]}", name, c.State, numRes, numServes, c.Defaults)
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
	if c.ServiceConfigs == nil {
		return errors.New("component without services set")
	}
	if c.Settings.SetupScript != "" && c.Settings.CleanupScript == "" {
		return errors.New("component has setup script but no cleanup script")
	}
	if c.Settings.SetupScript == "" && c.Settings.CleanupScript != "" {
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

func (c *Component) ToServiceState() (*services.ServiceSet, error) {
	result := services.NewServiceSet()
	for _, sc := range c.ServiceConfigs.List() {
		s := services.Service{
			Name:    sc.Service,
			State:   services.StateActive,
			Type:    "", // Do we even need this field
			Enabled: services.EnableStateEnabled,
		}
		if sc.Stopped {
			s.State = services.StateInternalWildcard
		}
		if sc.Disabled || !sc.Static {
			s.Enabled = services.EnableStateDisabled
		}
		result.Add(s)
	}
	return result, nil
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
	case ".conf":
		if IsDropin(file) {
			return ResourceTypeDropin
		}
		return ResourceTypeFile
	default:
		if len(file) == 0 {
			return ResourceTypeHost
		}
		if file[len(file)-1:] == "/" {
			if IsDropinDir(file) {
				return ResourceTypeDropinDir
			}
			return ResourceTypeDirectory
		}
		return ResourceTypeFile

	}
}

func IsDropin(file string) bool {
	if filepath.Dir(file) == "." {
		return false
	}
	return dropInDirRegex.MatchString(filepath.Dir(file))
}

func IsDropinDir(file string) bool {
	return dropInDirRegex.MatchString(file)
}

func IsTemplate(file string) bool {
	return strings.HasSuffix(file, ".gotmpl")
}
