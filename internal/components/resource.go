package components

import (
	"errors"
	"fmt"
)

type Resource struct {
	Path     string
	Name     string
	Parent   string
	Kind     ResourceType
	Template bool
}

//go:generate stringer -type ResourceType -trimprefix ResourceType
type ResourceType uint

const (
	ResourceTypeUnknown ResourceType = iota
	ResourceTypeContainer
	ResourceTypeVolume
	ResourceTypePod
	ResourceTypeNetwork
	ResourceTypeKube
	ResourceTypeFile
	ResourceTypeManifest
	ResourceTypeVolumeFile
	ResourceTypeScript
	ResourceTypeComponentScript
	ResourceTypeDirectory

	ResourceTypeService
)

func (r Resource) Validate() error {
	if r.Kind == ResourceTypeUnknown {
		return errors.New("unknown resource type")
	}
	if r.Name == "" {
		return errors.New("resource without name")
	}
	if r.Parent == "" {
		return errors.New("resource without parent component")
	}
	if r.Path == "" && r.Kind != ResourceTypeService {
		// TODO validate services properly
		return errors.New("resource without path")
	}
	return nil
}

func (r *Resource) String() string {
	return fmt.Sprintf("{r %v/%v %v %v %v }", r.Parent, r.Name, r.Path, r.Kind, r.Template)
}
