package components

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v5/pkg/systemd/parser"
)

type Resource struct {
	Path       string       `json:"path" toml:"path"`
	HostObject string       `json:"host_object" toml:"host_object"`
	Parent     string       `json:"parent" toml:"parent"`
	Kind       ResourceType `json:"kind" toml:"kind"`
	Template   bool         `json:"template" toml:"template"`
	Content    string
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
		return filepath.Base(fmt.Sprintf("%v.gotmpl", r.Path))
	}
	return filepath.Base(r.Path)
}

func (r *Resource) Filepath() string {
	if r.Template {
		return fmt.Sprintf("%v.gotmpl", r.Path)
	}
	return r.Path
}

func (r *Resource) Service() string {
	name := filepath.Base(r.Path)
	switch r.Kind {
	case ResourceTypeContainer:
		return strings.ReplaceAll(name, ".container", ".service")
	case ResourceTypeKube:
		return strings.ReplaceAll(name, ".kube", ".service")
	case ResourceTypePod:
		return strings.ReplaceAll(name, ".pod", "-pod.service")
	case ResourceTypeNetwork:
		return strings.ReplaceAll(name, ".network", "-network.service")
	case ResourceTypeVolume:
		return strings.ReplaceAll(name, ".volume", "-volume.service")
	case ResourceTypeBuild:
		return strings.ReplaceAll(name, ".build", "-build.service")
	case ResourceTypeImage:
		return strings.ReplaceAll(name, ".image", "-image.service")
	default:
		return ""
	}
}

func (r Resource) IsQuadlet() bool {
	switch r.Kind {
	case ResourceTypeContainer, ResourceTypeKube, ResourceTypeVolume, ResourceTypeNetwork, ResourceTypePod, ResourceTypeBuild, ResourceTypeImage:
		return true
	default:
		return false
	}
}

func (r Resource) IsFile() bool {
	switch r.Kind {
	case ResourceTypeContainer, ResourceTypeFile, ResourceTypeKube, ResourceTypeManifest, ResourceTypeNetwork, ResourceTypePod, ResourceTypeImage, ResourceTypeBuild, ResourceTypeScript, ResourceTypeVolume, ResourceTypeService, ResourceTypeComponentScript:
		return true
	default:
		return false
	}
}

func (r Resource) GetHostObject(unitData string) (string, error) {
	if !r.IsQuadlet() {
		return "", errors.New("can't get host object for non-quadlet")
	}
	unitfile := parser.NewUnitFile()
	err := unitfile.Parse(unitData)
	if err != nil {
		return "", fmt.Errorf("error parsing systemd unit file: %w", err)
	}
	nameOption := ""
	group := ""
	switch r.Kind {
	case ResourceTypeContainer:
		group = "Container"
		nameOption = "ContainerName"
	case ResourceTypeVolume:
		group = "Volume"
		nameOption = "VolumeName"
	case ResourceTypeNetwork:
		group = "Network"
		nameOption = "NetworkName"
	case ResourceTypePod:
		group = "Pod"
		nameOption = "PodName"
	case ResourceTypeBuild:
		group = "Build"
		nameOption = "ImageTag"
	case ResourceTypeImage:
		group = "Image"
		nameOption = "ImageTag"
	case ResourceTypeKube:
		group = "Kube"
		nameOption = "Yaml"
	}
	name, foundName := unitfile.Lookup(group, nameOption)
	if foundName {
		return name, nil
	}
	if r.Kind == ResourceTypeImage {
		name, ok := unitfile.Lookup(group, "Image")
		if !ok {
			return "", errors.New("something when horribly wrong with an image compo")
		}
		return name, nil
	}
	// Technically build and kube resources also don't have systemd- prefixed host objects
	// but we'll always have unique identifers for those quadlets so we won't worry about them yet.
	return fmt.Sprintf("systemd-%v", strings.TrimSuffix(filepath.Base(r.Path), filepath.Ext(r.Path))), nil
}
