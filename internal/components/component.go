package components

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"primamateria.systems/materia/internal/manifests"
)

var ErrCorruptComponent = errors.New("error corrupt component")

type Component struct {
	Name             string
	Resources        []Resource
	Scripted         bool
	State            ComponentLifecycle
	Defaults         map[string]any
	Secrets          []string
	VolumeResources  map[string]manifests.VolumeResourceConfig
	ServiceResources map[string]manifests.ServiceResourceConfig
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

func (c Component) Validate() error {
	if c.Name == "" {
		return errors.New("component without name")
	}
	if c.State == StateUnknown {
		return errors.New("component with unknown state")
	}
	return nil
}

func (c *Component) VersonData() (*bytes.Buffer, error) {
	vd := make(map[string]any)
	vd["Version"] = c.Version
	buffer, err := toml.Parser().Marshal(vd)
	if err != nil {
		return nil, errors.New("can't create version data")
	}
	return bytes.NewBuffer(buffer), nil
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
	case ".toml":
		return ResourceTypeManifest
	case ".service", ".timer", ".target":
		return ResourceTypeService
	case ".sh":
		return ResourceTypeScript
	default:
		fmt.Fprintf(os.Stderr, "FBLTHP[319]: component.go:92: file=%+v\n", file)
		if len(file) == 0 {
			return ResourceTypeHost
		}
		if file[len(file)-1:] == "/" {
			return ResourceTypeDirectory
		}
		return ResourceTypeFile

	}
}

func IsTemplate(file string) bool {
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
