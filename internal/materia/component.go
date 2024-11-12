package materia

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/secrets"
	"github.com/charmbracelet/log"
)

type Component struct {
	Name      string
	Services  []Resource
	Resources []Resource
	State     ComponentLifecycle
}

//go:generate stringer -type ComponentLifecycle -trimprefix State
type ComponentLifecycle int

const (
	StateUnknown ComponentLifecycle = iota
	StateStale
	StateFresh
	StateOK
	StateMayNeedUpdate
	StateNeedRemoval
	StateRemoved

	// Special states
	StateCanidate // a 'fake' component for resource comparison
)

func (c *Component) String() string {
	return fmt.Sprintf("{c %v %v }", c.Name, c.State)
}

func NewComponentFromSource(path string) (*Component, error) {
	c := &Component{}
	c.Name = filepath.Base(path)
	entries, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	var man *ComponentManifest
	resources := make(map[string]Resource)
	for _, v := range entries {
		resPath := filepath.Join(path, v.Name())
		if v.Name() == "MANIFEST.toml" {
			man, err = LoadComponentManifest(resPath)
			if err != nil {
				return nil, err
			}
		} else {
			newRes := Resource{
				Path:     resPath,
				Name:     strings.TrimSuffix(v.Name(), ".gotmpl"),
				Kind:     findResourceType(v.Name()),
				Template: isTemplate(v.Name()),
			}
			c.Resources = append(c.Resources, newRes)
			resources[newRes.Name] = newRes
		}
	}
	log.Debug(resources)
	if man != nil {
		for _, s := range man.Services {
			if s == "" {
				continue
			}
			resName := fmt.Sprintf("%v.container", s)
			_, ok := resources[resName]
			if !ok {
				log.Warn("tried to assign service to component but no matching resource", "component", c.Name, "service", resName)
			}
			c.Services = append(c.Services, Resource{
				Name: fmt.Sprintf("%v.service", s),
				Kind: ResourceTypeService,
			})
		}
	}

	return c, nil
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

func (c *Component) diff(other *Component, sm secrets.SecretsManager) ([]Action, error) {
	var diffActions []Action
	if len(c.Resources) == 0 || len(other.Resources) == 0 {
		log.Debug("components", "left", c, "right", other)
		return diffActions, fmt.Errorf("one or both components is missing resources: L:%v R:%v", len(c.Resources), len(other.Resources))
	}
	if err := c.Validate(); err != nil {
		return diffActions, fmt.Errorf("self component invalid during comparison: %w", err)
	}
	if err := other.Validate(); err != nil {
		return diffActions, fmt.Errorf("other component invalid during comparison: %w", err)
	}
	currentResources := make(map[string]Resource)
	newResources := make(map[string]Resource)
	for _, v := range c.Resources {
		currentResources[v.Name] = v
	}
	for _, v := range other.Resources {
		newResources[v.Name] = v
	}
	for k, cur := range currentResources {
		if newRes, ok := newResources[k]; ok {
			// check for diffs and update
			diffs, err := cur.diff(newRes, sm)
			if err != nil {
				return diffActions, err
			}
			if len(diffs) != 1 {
				log.Debug("updating current resource", "file", cur.Name)
				diffActions = append(diffActions, Action{
					Todo:    ActionUpdateResource,
					Parent:  c,
					Payload: newRes,
				})
			}
		} else {
			// in current resources but not source resources, remove old
			diffActions = append(diffActions, Action{
				Todo:    ActionRemoveResource,
				Parent:  c,
				Payload: cur,
			})
		}
	}

	for k, newRes := range newResources {
		if _, ok := currentResources[k]; !ok {
			// if new resource is not in old resource we need to install it
			log.Debug("installing new resource", "file", newRes.Name)
			diffActions = append(diffActions, Action{
				Todo:    ActionInstallResource,
				Parent:  c,
				Payload: newRes,
			})
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
	default:
		return ResourceTypeFile

	}
}

func isTemplate(file string) bool {
	return strings.HasSuffix(file, ".gotmpl")
}
