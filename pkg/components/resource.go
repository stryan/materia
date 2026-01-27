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

	ResourceTypeCombined

	ResourceTypeFile
	ResourceTypeManifest
	ResourceTypeScript
	ResourceTypeDirectory

	ResourceTypeService
	ResourceTypePodmanSecret
)

func (t ResourceType) toExt() (string, error) {
	switch t {
	case ResourceTypeBuild:
		return "build", nil
	case ResourceTypeComponent:
		return "component", nil
	case ResourceTypeContainer:
		return "container", nil
	case ResourceTypeImage:
		return "image", nil
	case ResourceTypeKube:
		return "kube", nil
	case ResourceTypeManifest:
		return "toml", nil
	case ResourceTypeNetwork:
		return "network", nil
	case ResourceTypePod:
		return "pod", nil
	case ResourceTypeScript:
		return "sh", nil
	case ResourceTypeVolume:
		return "volume", nil
	default:
		return "", fmt.Errorf("resource type wouldn't have file extension: %v", t)
	}
}

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
	case ResourceTypeService:
		return r.Path
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
	case ResourceTypeContainer, ResourceTypeFile, ResourceTypeKube, ResourceTypeManifest, ResourceTypeNetwork, ResourceTypePod, ResourceTypeImage, ResourceTypeBuild, ResourceTypeScript, ResourceTypeVolume, ResourceTypeService:
		return true
	default:
		return false
	}
}

func hostObjectFromUnitFile(r Resource, unitfile *parser.UnitFile) (string, error) {
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
			return "", errors.New("something when horribly wrong with an image comp")
		}
		return name, nil
	}
	// Technically build and kube resources also don't have systemd- prefixed host objects
	// but we'll always have unique identifers for those quadlets so we won't worry about them yet.
	return fmt.Sprintf("systemd-%v", strings.TrimSuffix(filepath.Base(r.Path), filepath.Ext(r.Path))), nil
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
	return hostObjectFromUnitFile(r, unitfile)
}

func GetResourcesFromQuadletsFile(parent, quadletData string) ([]Resource, error) {
	var result []Resource
	chunks := strings.Split(quadletData, "---\n")
	for _, quadlet := range chunks {
		res := Resource{}
		filename := ""
		firstLineIndex := strings.Index(quadlet, "\n")
		if firstLineIndex == -1 {
			return result, errors.New("something has gone horrible wrong")
		}
		firstLine := quadlet[:firstLineIndex]
		if strings.HasPrefix(firstLine, "# FileName") {
			name := strings.Split(firstLine, "=")
			if len(name) < 2 || name[1] == "" {
				return result, errors.New("bad filename")
			}
			filename = name[1]
		}
		if filename == "MANIFEST.toml" {
			res = Resource{
				Path:       "MANIFEST.toml",
				HostObject: "",
				Parent:     parent,
				Kind:       ResourceTypeManifest,
				Template:   false,
				Content:    quadlet,
			}
			continue
		}

		unitfile := parser.NewUnitFile()
		err := unitfile.Parse(quadlet)
		if err != nil {
			return result, fmt.Errorf("error parsing systemd unit file: %w", err)
		}
		if unitfile.HasGroup("Container") {
			res.Kind = ResourceTypeContainer
		}
		if unitfile.HasGroup("Volume") {
			res.Kind = ResourceTypeVolume
		}
		if unitfile.HasGroup("Build") {
			res.Kind = ResourceTypeBuild
		}
		if unitfile.HasGroup("Image") {
			res.Kind = ResourceTypeImage
		}
		if unitfile.HasGroup("Network") {
			res.Kind = ResourceTypeNetwork
		}
		if unitfile.HasGroup("Pod") {
			res.Kind = ResourceTypePod
		}
		if unitfile.HasGroup("Kube") {
			res.Kind = ResourceTypeKube
		}
		if res.Kind == ResourceTypeUnknown {
			// it's a valid systemd unit file but not a quadlet, treat it as a service
			res.Kind = ResourceTypeService
		}
		res.Content = quadlet[firstLineIndex:]
		res.HostObject, err = hostObjectFromUnitFile(res, unitfile)
		if err != nil {
			return result, err
		}
		res.Parent = parent
		ext, err := res.Kind.toExt()
		if err != nil {
			return result, err
		}
		res.Path = fmt.Sprintf("%v.%v", filename, ext)

		result = append(result, res)
	}
	return result, nil
}
