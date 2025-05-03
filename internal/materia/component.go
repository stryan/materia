package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/repository"
	"github.com/charmbracelet/log"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/sergi/go-diff/diffmatchpatch"
)

var errCorruptComponent = errors.New("error corrupt component")

type Component struct {
	Name             string
	Resources        []Resource
	Scripted         bool
	State            ComponentLifecycle
	Defaults         map[string]any
	VolumeResources  map[string]VolumeResourceConfig
	ServiceResources map[string]ServiceResourceConfig
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

func NewComponentFromSource(path string) (*Component, error) {
	c := &Component{}
	c.Name = filepath.Base(path)
	c.Defaults = make(map[string]any)
	c.Version = DefaultComponentVersion
	c.VolumeResources = make(map[string]VolumeResourceConfig)
	c.ServiceResources = make(map[string]ServiceResourceConfig)
	log.Debugf("loading component %v from path %v", c.Name, path)
	entries, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	var man *ComponentManifest
	scripts := 0
	for _, v := range entries {
		resPath := filepath.Join(path, v.Name())
		if v.Name() == "MANIFEST.toml" {
			log.Debugf("loading source component manifest %v", c.Name)
			man, err = LoadComponentManifest(resPath)
			if err != nil {
				return nil, fmt.Errorf("error loading component manifest: %w", err)
			}
			maps.Copy(c.Defaults, man.Defaults)
			maps.Copy(c.VolumeResources, man.VolumeResources)
			continue
		}
		var newRes Resource
		if v.Name() == "setup.sh" || v.Name() == "cleanup.sh" {
			scripts++
			c.Scripted = true
			newRes = Resource{
				Path:     resPath,
				Name:     v.Name(),
				Kind:     ResourceTypeComponentScript,
				Template: false,
			}
		} else {
			newRes = Resource{
				Path:     resPath,
				Name:     strings.TrimSuffix(v.Name(), ".gotmpl"),
				Kind:     findResourceType(v.Name()),
				Template: isTemplate(v.Name()),
			}
		}
		for _, vr := range c.VolumeResources {
			if vr.Resource == newRes.Name {
				newRes.Kind = ResourceTypeVolumeFile
			}
		}
		c.Resources = append(c.Resources, newRes)
	}
	if man == nil {
		return nil, errCorruptComponent
	}
	if scripts != 0 && scripts != 2 {
		return nil, errors.New("scripted component is missing install or cleanup")
	}
	for _, s := range man.Services {
		if err := s.Validate(); err != nil {
			return nil, fmt.Errorf("invalid service for component: %w", err)
		}
		c.ServiceResources[s.Service] = s
	}

	c.Resources = append(c.Resources, Resource{
		Path:     filepath.Join(path, "MANIFEST.toml"),
		Name:     "MANIFEST.toml",
		Kind:     ResourceTypeManifest,
		Template: false,
	})
	for k, r := range c.Resources {
		if r.Kind != ResourceTypeScript && slices.Contains(man.Scripts, r.Name) {
			r.Kind = ResourceTypeScript
			c.Resources[k] = r
		}
	}

	return c, nil
}

func NewComponentFromHost(name string, compRepo *repository.HostComponentRepository) (*Component, error) {
	oldComp := &Component{
		Name:             name,
		Resources:        []Resource{},
		State:            StateStale,
		Defaults:         make(map[string]any),
		VolumeResources:  make(map[string]VolumeResourceConfig),
		ServiceResources: make(map[string]ServiceResourceConfig),
	}
	// load resources
	ctx := context.Background()
	var man *ComponentManifest
	entries, err := compRepo.ListResources(ctx, name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errCorruptComponent
		}
		return nil, fmt.Errorf("error listing resources: %w", err)
	}
	versionFileExists, err := compRepo.Exists(ctx, filepath.Join(name, ".component_version"))
	if err != nil {
		return nil, errCorruptComponent
	}
	if versionFileExists {
		k := koanf.New(".")
		err := k.Load(file.Provider(".component_version"), toml.Parser())
		if err != nil {
			return nil, err
		}
		var c ComponentVersion
		err = k.Unmarshal("", &c)
		if err != nil {
			return nil, err
		}
		oldComp.Version = c.Version
	} else {
		oldComp.Version = -1
	}
	scripts := 0
	manifestFound := false
	for _, e := range entries {
		resName := filepath.Base(e)
		newRes := Resource{
			Path:     e,
			Name:     strings.TrimSuffix(resName, ".gotmpl"),
			Kind:     findResourceType(resName),
			Template: isTemplate(resName),
		}

		oldComp.Resources = append(oldComp.Resources, newRes)
		if resName == "MANIFEST.toml" && oldComp.Version == DefaultComponentVersion {
			manifestFound = true
			log.Debugf("loading installed component manifest %v", oldComp.Name)
			man, err = LoadComponentManifest(newRes.Path)
			if err != nil {
				return nil, fmt.Errorf("error loading component manifest: %w", err)
			}
			maps.Copy(oldComp.Defaults, man.Defaults)
			for _, s := range man.Services {
				if err := s.Validate(); err != nil {
					return nil, fmt.Errorf("invalid service for component: %w", err)
				}
				oldComp.ServiceResources[s.Service] = s
			}
			maps.Copy(oldComp.VolumeResources, man.VolumeResources)
		}
		if resName == "setup.sh" || resName == "cleanup.sh" {
			scripts++
			oldComp.Scripted = true
		}

	}
	if !manifestFound {
		return nil, errCorruptComponent
	}
	if scripts != 0 && scripts != 2 {
		return nil, errors.New("scripted component is missing install or cleanup")
	}
	for k, r := range oldComp.Resources {
		if man != nil && r.Kind != ResourceTypeScript && slices.Contains(man.Scripts, r.Name) {
			r.Kind = ResourceTypeScript
			oldComp.Resources[k] = r
		}
	}

	return oldComp, nil
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

func (c *Component) test(_ context.Context, fmap MacroMap, vars map[string]any) error {
	diffVars := make(map[string]any)
	maps.Copy(diffVars, c.Defaults)
	maps.Copy(diffVars, vars)
	for _, newRes := range c.Resources {
		_, err := newRes.execute(fmap, diffVars)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Component) diff(other *Component, fmap MacroMap, vars map[string]any) ([]Action, error) {
	var diffActions []Action
	if len(other.Resources) == 0 {
		log.Debug("components", "left", c, "right", other)
		return diffActions, fmt.Errorf("candidate component is missing resources: L:%v R:%v", len(c.Resources), len(other.Resources))
	}
	if err := c.Validate(); err != nil {
		return diffActions, fmt.Errorf("self component invalid during comparison: %w", err)
	}
	if err := other.Validate(); err != nil {
		return diffActions, fmt.Errorf("other component invalid during comparison: %w", err)
	}
	currentResources := make(map[string]Resource)
	newResources := make(map[string]Resource)
	diffVars := make(map[string]any)
	maps.Copy(diffVars, c.Defaults)
	maps.Copy(diffVars, other.Defaults)
	maps.Copy(diffVars, vars)
	for _, v := range c.Resources {
		currentResources[v.Name] = v
	}
	for _, v := range other.Resources {
		newResources[v.Name] = v
	}

	keys := sortedKeys(currentResources)
	for _, k := range keys {
		cur := currentResources[k]
		if newRes, ok := newResources[k]; ok {
			// check for diffs and update
			log.Debug("diffing resource", "component", c.Name, "file", cur.Name)
			diffs, err := cur.diff(fmap, newRes, diffVars)
			if err != nil {
				return diffActions, err
			}
			if len(diffs) < 1 {
				// comparing empty files
				continue
			}
			if len(diffs) > 1 || diffs[0].Type != diffmatchpatch.DiffEqual {
				log.Debug("updating current resource", "file", cur.Name, "diffs", diffs)
				a := Action{
					Todo:    newRes.toAction("update"),
					Parent:  c,
					Payload: newRes,
					Content: diffs,
				}

				diffActions = append(diffActions, a)
			}
		} else {
			// in current resources but not source resources, remove old
			log.Debug("removing current resource", "file", cur.Name)
			a := Action{
				Todo:    newRes.toAction("remove"),
				Parent:  c,
				Payload: cur,
			}

			diffActions = append(diffActions, a)
		}
	}
	keys = sortedKeys(newResources)
	for _, k := range keys {
		if _, ok := currentResources[k]; !ok {
			// if new resource is not in old resource we need to install it
			fmt.Printf("Creating new resource %v", k)
			a := Action{
				Todo:    newResources[k].toAction("install"),
				Parent:  c,
				Payload: newResources[k],
			}
			diffActions = append(diffActions, a)
		}
	}

	return diffActions, nil
}

func (c *Component) VersonData() (*bytes.Buffer, error) {
	vd := make(map[string]interface{})
	vd["Version"] = c.Version
	buffer, err := toml.Parser().Marshal(vd)
	if err != nil {
		return nil, errors.New("can't create version data")
	}
	return bytes.NewBuffer(buffer), nil
}

func findResourceType(file string) ResourceType {
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

func isTemplate(file string) bool {
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
