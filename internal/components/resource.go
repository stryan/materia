package components

import (
	"errors"
	"fmt"
)

type Resource struct {
	Path     string
	Parent   string
	Kind     ResourceType
	Template bool
}

//go:generate stringer -type ResourceType -trimprefix ResourceType
type ResourceType uint

const (
	ResourceTypeUnknown ResourceType = iota

	ResourceTypeComponent
	ResourceTypeHost

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
	ResourceTypePodmanSecret
)

func (r Resource) Validate() error {
	if r.Kind == ResourceTypeUnknown {
		return errors.New("unknown resource type")
	}
	if r.Kind == ResourceTypeHost && r.Path != "" {
		return errors.New("can't name host")
	}
	if r.Path == "" {
		return errors.New("resource without name")
	}
	if r.Parent == "" {
		return errors.New("resource without parent component")
	}
	return nil
}

func (r *Resource) String() string {
	return fmt.Sprintf("{r %v/%v %v %v }", r.Parent, r.Path, r.Kind, r.Template)
}

func (r *Resource) Name() string {
	if r.Template {
		return fmt.Sprintf("%v.gotmpl", r.Path)
	}
	return r.Path
}

func (r Resource) IsQuadlet() bool {
	switch r.Kind {
	case ResourceTypeContainer, ResourceTypeKube, ResourceTypeVolume, ResourceTypeNetwork, ResourceTypePod:
		return true
	default:
		return false
	}
}
