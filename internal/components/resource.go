package components

import (
	"errors"
	"fmt"
)

type Resource struct {
	Path       string       `json:"path" toml:"path"`
	HostObject string       `json:"host_object" toml:"host_object"`
	Parent     string       `json:"parent" toml:"parent"`
	Kind       ResourceType `json:"kind" toml:"kind"`
	Template   bool         `json:"template" toml:"template"`
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
	ResourceTypeBuild
	ResourceTypeImage

	ResourceTypeFile
	ResourceTypeManifest
	ResourceTypeScript
	ResourceTypeComponentScript
	ResourceTypeDirectory

	ResourceTypeService
	ResourceTypePodmanSecret
)

func (r Resource) Validate() error {
	if r.Kind == ResourceTypeHost {
		if r.Path != "" {
			return errors.New("can't name host resource")
		}
		return nil
	}
	if r.Kind == ResourceTypeUnknown {
		return errors.New("unknown resource type")
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

func (r Resource) IsFile() bool {
	switch r.Kind {
	case ResourceTypeContainer, ResourceTypeFile, ResourceTypeKube, ResourceTypeManifest, ResourceTypeNetwork, ResourceTypePod, ResourceTypeScript, ResourceTypeVolume, ResourceTypeService, ResourceTypeComponentScript:
		return true
	default:
		return false
	}
}
