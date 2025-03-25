package materia

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/repository"
	"github.com/charmbracelet/log"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type Component struct {
	Name            string
	Services        []Resource
	Resources       []Resource
	Scripted        bool
	State           ComponentLifecycle
	Defaults        map[string]interface{}
	VolumeResources map[string]VolumeResourceConfig
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

func (c *Component) String() string {
	return fmt.Sprintf("{c %v %v Rs: %v D: [%v]}", c.Name, c.State, len(c.Resources), c.Defaults)
}

func NewComponentFromSource(path string) (*Component, error) {
	c := &Component{}
	c.Name = filepath.Base(path)
	c.Defaults = make(map[string]interface{})
	c.VolumeResources = make(map[string]VolumeResourceConfig)
	log.Debugf("loading component %v from path %v", c.Name, path)
	entries, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	var man *ComponentManifest
	resources := make(map[string]Resource)
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
		resources[newRes.Name] = newRes
	}
	if scripts != 0 && scripts != 2 {
		return nil, errors.New("scripted component is missing install or cleanup")
	}
	if man != nil {
		if !man.NoServices {
			for _, s := range man.Services {
				if s == "" || (!strings.HasSuffix(s, ".service") && !strings.HasSuffix(s, ".target") && !strings.HasSuffix(s, ".timer")) {
					return nil, fmt.Errorf("error loading component services: invalid format %v", s)
				}
				c.Services = append(c.Services, Resource{
					Name: s,
					Kind: ResourceTypeService,
				})
			}
		}
	} else {
		for _, r := range c.Resources {
			if r.Kind == ResourceTypeContainer || r.Kind == ResourceTypePod {
				serv, err := r.getServiceFromResource()
				if err != nil {
					return nil, fmt.Errorf("error guestimating component service: %w", err)
				}
				c.Services = append(c.Services, serv)
			}
		}
	}

	return c, nil
}

func NewComponentFromHost(name string, compRepo *repository.ComponentRepository) (*Component, error) {
	oldComp := &Component{
		Name:            name,
		Resources:       []Resource{},
		State:           StateStale,
		Services:        []Resource{},
		Defaults:        make(map[string]interface{}),
		VolumeResources: make(map[string]VolumeResourceConfig),
	}
	// load resources

	var man *ComponentManifest
	entries, err := compRepo.ListResources(context.Background(), name)
	if err != nil {
		return nil, fmt.Errorf("error listing resources: %w", err)
	}
	scripts := 0
	for _, e := range entries {
		resName := filepath.Base(e)
		newRes := Resource{
			Path:     e,
			Name:     strings.TrimSuffix(resName, ".gotmpl"),
			Kind:     findResourceType(resName),
			Template: isTemplate(resName),
		}

		oldComp.Resources = append(oldComp.Resources, newRes)
		if resName == "MANIFEST.toml" {
			log.Debugf("loading installed component manifest %v", oldComp.Name)
			man, err = LoadComponentManifest(newRes.Path)
			if err != nil {
				return nil, fmt.Errorf("error loading component manifest: %w", err)
			}
			maps.Copy(oldComp.Defaults, man.Defaults)
			if !man.NoServices {
				for _, s := range man.Services {
					if s == "" || (!strings.HasSuffix(s, ".service") && !strings.HasSuffix(s, ".target") && !strings.HasSuffix(s, ".timer")) {
						return nil, fmt.Errorf("error loading component services: invalid format %v", s)
					}
					oldComp.Services = append(oldComp.Services, Resource{
						Name: s,
						Kind: ResourceTypeService,
					})
				}
			}
			maps.Copy(oldComp.VolumeResources, man.VolumeResources)
		}
		if resName == "setup.sh" || resName == "cleanup.sh" {
			scripts++
			oldComp.Scripted = true
		}

	}

	if scripts != 0 && scripts != 2 {
		return nil, errors.New("scripted component is missing install or cleanup")
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

func (c *Component) test(_ context.Context, fmap MacroMap, vars map[string]interface{}) error {
	diffVars := make(map[string]interface{})
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

func (c *Component) diff(other *Component, fmap MacroMap, vars map[string]interface{}) ([]Action, error) {
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
	diffVars := make(map[string]interface{})
	maps.Copy(diffVars, c.Defaults)
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
			log.Debug("installing new resource", "file", k)
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
	default:
		return ResourceTypeFile

	}
}

func isTemplate(file string) bool {
	return strings.HasSuffix(file, ".gotmpl")
}
