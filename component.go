package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type Component struct {
	Name      string
	Services  []string
	Resources []Resource
	State     ComponentLifecycle
}

type ComponentLifecycle int

const (
	StateUnknown ComponentLifecycle = iota
	StateStale
	StateFresh
	StateOK
	StateMayNeedUpdate
	StateNeedRemoval
	StateRemoved
)

func NewComponent(path string) *Component {
	d := &Component{}
	d.Name = filepath.Base(path)
	entries, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range entries {
		newRes := Resource{
			Path:     filepath.Join(path, v.Name()),
			Name:     strings.TrimSuffix(v.Name(), ".gotmpl"),
			Kind:     FindResourceType(v.Name()),
			Template: isTemplate(v.Name()),
		}
		d.Resources = append(d.Resources, newRes)
	}
	for _, v := range d.Resources {
		if v.Kind == ResourceTypeContainer || v.Kind == ResourceTypePod {
			d.Services = append(d.Services, fmt.Sprintf("%v.service", strings.TrimSuffix(v.Name, ".container")))
		}
	}
	return d
}

func (d *Component) ServiceForResource(_ Resource) []string {
	return d.Services
}

func (c *Component) Diff(other *Component) ([]Action, error) {
	var diffActions []Action
	dmp := diffmatchpatch.New()
	if len(c.Resources) == 0 || len(other.Resources) == 0 {
		log.Debug("components", "left", c, "right", other)
		return diffActions, fmt.Errorf("one or both components is missing resources: L:%v R:%v", len(c.Resources), len(other.Resources))
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
			curFile, err := os.ReadFile(cur.Path)
			if err != nil {
				return diffActions, err
			}
			newFile, err := os.ReadFile(newRes.Path)
			if err != nil {
				return diffActions, err
			}
			diffs := dmp.DiffMain(string(curFile), string(newFile), false)
			if len(diffs) != 0 {
				diffActions = append(diffActions, Action{
					Todo:    ActionUpdateResource,
					Payload: []string{c.Name, newRes.Name},
				})
			}
		} else {
			// in current resources but not source resources, remove old
			diffActions = append(diffActions, Action{
				Todo:    ActionRemoveResource,
				Payload: []string{c.Name, cur.Name},
			})
		}
	}

	for k := range newResources {
		if _, ok := currentResources[k]; !ok {
			// if new resource is not in old resource we need to install it
			diffActions = append(diffActions, Action{
				Todo:    ActionInstallResource,
				Payload: []string{c.Name, k},
			})
		}
	}

	return diffActions, nil
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
	default:
		return ResourceTypeFile

	}
}

func isTemplate(file string) bool {
	return strings.HasSuffix(file, ".gotmpl")
}
