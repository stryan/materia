package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/secrets"
	"github.com/charmbracelet/log"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type Component struct {
	Name      string
	Services  []Resource
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

func NewComponentFromSource(path string) *Component {
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
	return d
}

func (c *Component) Diff(other *Component, sm secrets.SecretsManager) ([]Action, error) {
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
			curString := string(curFile)
			// parse if template
			newFile, err := os.ReadFile(newRes.Path)
			if err != nil {
				return diffActions, err
			}
			var newString string
			result := bytes.NewBuffer([]byte{})
			if newRes.Template {
				tmpl, err := template.New(newRes.Name).Parse(string(newFile))
				if err != nil {
					return diffActions, err
				}
				err = tmpl.Execute(result, sm.Lookup(context.Background(), secrets.SecretFilter{}))
				if err != nil {
					return diffActions, err
				}

			} else {
				result = bytes.NewBuffer(newFile)
			}
			newString = result.String()
			diffs := dmp.DiffMain(curString, newString, false)
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
