package components

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/manifests"
	"github.com/knadh/koanf/parsers/toml"
)

var ErrCorruptComponent = errors.New("error corrupt component")

type Component struct {
	Name             string
	Resources        []Resource
	Scripted         bool
	State            ComponentLifecycle
	Defaults         map[string]any
	VolumeResources  map[string]manifests.VolumeResourceConfig
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

const DefaultComponentVersion = 1

func (c *Component) String() string {
	return fmt.Sprintf("{c %v %v Rs: %v Ss: %v D: [%v]}", c.Name, c.State, len(c.Resources), len(c.ServiceResources), c.Defaults)
}

// func NewComponentFromSource(path string) (*Component, error) {
// 	c := &Component{}
// 	c.Name = filepath.Base(path)
// 	c.Defaults = make(map[string]any)
// 	c.Version = DefaultComponentVersion
// 	c.VolumeResources = make(map[string]manifests.VolumeResourceConfig)
// 	c.ServiceResources = make(map[string]manifests.ServiceResourceConfig)
// 	log.Debugf("loading component %v from path %v", c.Name, path)
// 	var man *manifests.ComponentManifest
// 	scripts := 0
// 	err := filepath.WalkDir(path, func(fullPath string, d fs.DirEntry, err error) error {
// 		if err != nil {
// 			return err
// 		}
// 		if d.Name() == c.Name {
// 			return nil
// 		}
// 		resPath := strings.TrimPrefix(fullPath, path)
// 		if d.Name() == "MANIFEST.toml" {
// 			log.Debugf("loading source component manifest %v", c.Name)
// 			man, err = manifests.LoadComponentManifest(fullPath)
// 			if err != nil {
// 				return fmt.Errorf("error loading component manifest: %w", err)
// 			}
// 			maps.Copy(c.Defaults, man.Defaults)
// 			maps.Copy(c.VolumeResources, man.VolumeResources)
// 			return nil
// 		}
// 		var newRes Resource
// 		if d.Name() == "setup.sh" || d.Name() == "cleanup.sh" {
// 			scripts++
// 			c.Scripted = true
// 			newRes = Resource{
// 				Path:     resPath,
// 				Name:     d.Name(),
// 				Parent:   c.Name,
// 				Kind:     ResourceTypeComponentScript,
// 				Template: false,
// 			}
// 		} else {
// 			newRes = Resource{
// 				Path:     resPath,
// 				Parent:   c.Name,
// 				Name:     strings.TrimSuffix(d.Name(), ".gotmpl"),
// 				Kind:     FindResourceType(d.Name()),
// 				Template: IsTemplate(d.Name()),
// 			}
// 		}
// 		for _, vr := range c.VolumeResources {
// 			if vr.Resource == newRes.Name {
// 				newRes.Kind = ResourceTypeVolumeFile
// 			}
// 		}
// 		c.Resources = append(c.Resources, newRes)
// 		return nil
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	if man == nil {
// 		return nil, ErrCorruptComponent
// 	}
// 	if scripts != 0 && scripts != 2 {
// 		return nil, errors.New("scripted component is missing install or cleanup")
// 	}
// 	for _, s := range man.Services {
// 		if err := s.Validate(); err != nil {
// 			return nil, fmt.Errorf("invalid service for component: %w", err)
// 		}
// 		c.ServiceResources[s.Service] = s
// 	}
//
// 	c.Resources = append(c.Resources, Resource{
// 		Parent:   c.Name,
// 		Path:     "/MANIFEST.toml",
// 		Name:     "MANIFEST.toml",
// 		Kind:     ResourceTypeManifest,
// 		Template: false,
// 	})
// 	for k, r := range c.Resources {
// 		if r.Kind != ResourceTypeScript && slices.Contains(man.Scripts, r.Name) {
// 			r.Kind = ResourceTypeScript
// 			c.Resources[k] = r
// 		}
// 	}
//
// 	return c, nil
// }
//
// func NewComponentFromHost(name string, compRepo *repository.HostComponentRepository) (*Component, error) {
// 	oldComp := &Component{
// 		Name:             name,
// 		Resources:        []Resource{},
// 		State:            StateStale,
// 		Defaults:         make(map[string]any),
// 		VolumeResources:  make(map[string]manifests.VolumeResourceConfig),
// 		ServiceResources: make(map[string]manifests.ServiceResourceConfig),
// 	}
// 	// load resources
// 	ctx := context.Background()
// 	var man *manifests.ComponentManifest
// 	entries, err := compRepo.ListResources(ctx, name)
// 	if err != nil {
// 		if os.IsNotExist(err) {
// 			return nil, fmt.Errorf("%w: missing component %v data", ErrCorruptComponent, name)
// 		}
// 		return nil, fmt.Errorf("error listing resources: %w", err)
// 	}
// 	versionFileExists, err := compRepo.Exists(ctx, filepath.Join(name, ".component_version"))
// 	if err != nil {
// 		return nil, err
// 	}
// 	if versionFileExists {
// 		k := koanf.New(".")
// 		// TODO don't leak DataPrefix?
// 		err := k.Load(file.Provider(filepath.Join(compRepo.DataPrefix, name, ".component_version")), toml.Parser())
// 		if err != nil {
// 			return nil, err
// 		}
// 		var c ComponentVersion
// 		err = k.Unmarshal("", &c)
// 		if err != nil {
// 			return nil, err
// 		}
// 		oldComp.Version = c.Version
// 	} else {
// 		oldComp.Version = -1
// 	}
// 	scripts := 0
// 	manifestFound := false
// 	// TODO switch to walk
// 	for _, e := range entries {
// 		resName := filepath.Base(e)
// 		newRes := Resource{
// 			Parent:   name,
// 			Path:     e,
// 			Name:     strings.TrimSuffix(resName, ".gotmpl"),
// 			Kind:     FindResourceType(resName),
// 			Template: IsTemplate(resName),
// 		}
//
// 		oldComp.Resources = append(oldComp.Resources, newRes)
// 		if resName == "MANIFEST.toml" {
// 			manifestFound = true
// 			if oldComp.Version == DefaultComponentVersion {
// 				log.Debugf("loading installed component manifest %v", oldComp.Name)
// 				man, err = manifests.LoadComponentManifest(newRes.Path)
// 				if err != nil {
// 					return nil, fmt.Errorf("error loading component manifest: %w", err)
// 				}
// 				maps.Copy(oldComp.Defaults, man.Defaults)
// 				for _, s := range man.Services {
// 					if err := s.Validate(); err != nil {
// 						return nil, fmt.Errorf("invalid service for component: %w", err)
// 					}
// 					oldComp.ServiceResources[s.Service] = s
// 				}
// 				maps.Copy(oldComp.VolumeResources, man.VolumeResources)
// 			}
// 		}
// 		if resName == "setup.sh" || resName == "cleanup.sh" {
// 			scripts++
// 			oldComp.Scripted = true
// 		}
//
// 	}
// 	if !manifestFound {
// 		return nil, ErrCorruptComponent
// 	}
// 	if scripts != 0 && scripts != 2 {
// 		return nil, errors.New("scripted component is missing install or cleanup")
// 	}
// 	for k, r := range oldComp.Resources {
// 		if man != nil && r.Kind != ResourceTypeScript && slices.Contains(man.Scripts, r.Name) {
// 			r.Kind = ResourceTypeScript
// 			oldComp.Resources[k] = r
// 		}
// 	}
//
// 	return oldComp, nil
// }

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
	case ".kube":
		return ResourceTypeKube
	case ".toml":
		return ResourceTypeManifest
	case ".service", ".timer", ".target":
		return ResourceTypeService
	case ".sh":
		return ResourceTypeScript
	default:
		return ResourceTypeFile

	}
}

func IsTemplate(file string) bool {
	return strings.HasSuffix(file, ".gotmpl")
}

// func isQuadlet(file string) bool {
// 	filename := strings.TrimSuffix(file, ".gotmpl")
// 	switch filepath.Ext(filename) {
// 	case ".pod", ".container", ".network", ".volume", ".kube":
// 		return true
// 	}
// 	return false
// }
