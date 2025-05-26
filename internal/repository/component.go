package repository

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/components"
	"git.saintnet.tech/stryan/materia/internal/manifests"
	"github.com/charmbracelet/log"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type ComponentRespository interface {
	GetComponent(string) (*components.Component, error)
	GetResource(*components.Component, string) (components.Resource, error)
	GetManifest(*components.Component) (*manifests.ComponentManifest, error)
	InstallComponent(*components.Component) error
	ComponentExists(string) (bool, error)
	RemoveComponent(*components.Component) error
	ReadResource(components.Resource) (string, error)
	InstallResource(components.Resource, *bytes.Buffer) error
	RemoveResource(components.Resource) error
	ListResources(*components.Component) ([]components.Resource, error)
	ListComponentNames() ([]string, error)
	RunCleanup(*components.Component) error
	RunSetup(*components.Component) error
	PurgeComponent(*components.Component) error
	PurgeComponentByName(string) error
	Clean() error
}

type HostComponentRepository struct {
	DataPrefix    string
	QuadletPrefix string
}

func (r *HostComponentRepository) GetComponent(name string) (*components.Component, error) {
	oldComp := &components.Component{
		Name:             name,
		Resources:        []components.Resource{},
		State:            components.StateStale,
		Defaults:         make(map[string]any),
		VolumeResources:  make(map[string]manifests.VolumeResourceConfig),
		ServiceResources: make(map[string]manifests.ServiceResourceConfig),
	}
	dataPath := filepath.Join(r.DataPrefix, name)
	quadletPath := filepath.Join(r.QuadletPrefix, name)
	// load resources
	var man *manifests.ComponentManifest
	_, err := os.Stat(filepath.Join(dataPath, ".component_version"))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error reading component version: %w", err)
	}
	versionFileExists := os.IsExist(err)
	if versionFileExists {
		k := koanf.New(".")
		err := k.Load(file.Provider(filepath.Join(dataPath, ".component_version")), toml.Parser())
		if err != nil {
			return nil, err
		}
		var c components.ComponentVersion
		err = k.Unmarshal("", &c)
		if err != nil {
			return nil, err
		}
		oldComp.Version = c.Version
	} else {
		oldComp.Version = -1
	}
	scripts := 0
	manifestFound := false
	err = filepath.WalkDir(dataPath, func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == oldComp.Name || d.Name() == ".component_version" {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		resPath := strings.TrimPrefix(fullPath, dataPath)
		resName := filepath.Base(fullPath)
		newRes := components.Resource{
			Parent:   name,
			Path:     resPath,
			Name:     resName,
			Kind:     components.FindResourceType(resName),
			Template: components.IsTemplate(resName),
		}
		oldComp.Resources = append(oldComp.Resources, newRes)
		if resName == "MANIFEST.toml" {
			manifestFound = true
			if oldComp.Version == components.DefaultComponentVersion {
				log.Debugf("loading installed component manifest %v", oldComp.Name)
				man, err = manifests.LoadComponentManifest(newRes.Path)
				if err != nil {
					return fmt.Errorf("error loading component manifest: %w", err)
				}
				maps.Copy(oldComp.Defaults, man.Defaults)
				for _, s := range man.Services {
					if err := s.Validate(); err != nil {
						return fmt.Errorf("invalid service for component: %w", err)
					}
					oldComp.ServiceResources[s.Service] = s
				}
				maps.Copy(oldComp.VolumeResources, man.VolumeResources)
			}
		}
		if resName == "setup.sh" || resName == "cleanup.sh" {
			scripts++
			oldComp.Scripted = true
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(quadletPath, func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == oldComp.Name || d.Name() == ".materia_managed" {
			return nil
		}
		resPath := strings.TrimPrefix(fullPath, dataPath)
		resName := filepath.Base(fullPath)
		newRes := components.Resource{
			Parent:   name,
			Path:     resPath,
			Name:     resName,
			Kind:     components.FindResourceType(resName),
			Template: components.IsTemplate(resName),
		}
		oldComp.Resources = append(oldComp.Resources, newRes)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if !manifestFound {
		return nil, components.ErrCorruptComponent
	}
	if scripts != 0 && scripts != 2 {
		return nil, errors.New("scripted component is missing install or cleanup")
	}
	for k, r := range oldComp.Resources {
		if man != nil && r.Kind != components.ResourceTypeScript && slices.Contains(man.Scripts, r.Name) {
			r.Kind = components.ResourceTypeScript
			oldComp.Resources[k] = r
		}
	}

	return oldComp, nil
}

func (r *HostComponentRepository) GetManifest(parent *components.Component) (*manifests.ComponentManifest, error) {
	return manifests.LoadComponentManifest(filepath.Join(r.DataPrefix, parent.Name, "MANIFEST.toml"))
}

func (r *HostComponentRepository) GetResource(parent *components.Component, name string) (components.Resource, error) {
	if parent == nil || name == "" {
		return components.Resource{}, errors.New("invalid parent or resource")
	}
	dataPath := filepath.Join(r.DataPrefix, parent.Name)
	quadletPath := filepath.Join(r.QuadletPrefix, parent.Name)
	resourcePath := ""
	breakWalk := false
	searchFunc := func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if breakWalk {
			return nil
		}
		if d.Name() == parent.Name || d.Name() == ".component_version" || d.Name() == ".materia_managed" {
			return nil
		}
		if d.Name() == name {
			resourcePath = fullPath
			breakWalk = true
			return nil
		}
		return nil
	}
	err := filepath.WalkDir(dataPath, searchFunc)
	if err != nil {
		return components.Resource{}, err
	}
	err = filepath.WalkDir(quadletPath, searchFunc)
	if err != nil {
		return components.Resource{}, err
	}
	if resourcePath == "" {
		return components.Resource{}, errors.New("resource not found")
	}
	resPath := strings.TrimPrefix(resourcePath, dataPath)
	resName := filepath.Base(resourcePath)
	return components.Resource{
		Parent:   name,
		Path:     resPath,
		Name:     resName,
		Kind:     components.FindResourceType(resName),
		Template: components.IsTemplate(resName),
	}, nil
}

func (r *HostComponentRepository) ListResources(c *components.Component) ([]components.Resource, error) {
	if c == nil {
		return []components.Resource{}, errors.New("invalid parent or resource")
	}
	resources := []components.Resource{}
	dataPath := filepath.Join(r.DataPrefix, c.Name)
	quadletPath := filepath.Join(r.QuadletPrefix, c.Name)
	searchFunc := func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Name() == c.Name || d.Name() == ".component_version" || d.Name() == ".materia_managed" {
			return nil
		}
		resPath := strings.TrimPrefix(fullPath, dataPath)
		resName := filepath.Base(fullPath)
		newRes := components.Resource{
			Parent:   c.Name,
			Path:     resPath,
			Name:     resName,
			Kind:     components.FindResourceType(resName),
			Template: components.IsTemplate(resName),
		}
		resources = append(resources, newRes)

		return nil
	}
	err := filepath.WalkDir(dataPath, searchFunc)
	if err != nil {
		return resources, err
	}
	err = filepath.WalkDir(quadletPath, searchFunc)
	if err != nil {
		return resources, err
	}
	return resources, nil
}

func (r *HostComponentRepository) ListComponentNames() ([]string, error) {
	var compPaths []string
	entries, err := os.ReadDir(r.DataPrefix)
	if err != nil {
		return nil, err
	}
	for _, v := range entries {
		if v.IsDir() {
			compPaths = append(compPaths, v.Name())
		}
	}
	return compPaths, nil
}

func (r *HostComponentRepository) InstallComponent(c *components.Component) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c == nil {
		return errors.New("invalid component")
	}
	err := os.Mkdir(filepath.Join(r.DataPrefix, c.Name), 0o755)
	if err != nil {
		return fmt.Errorf("error installing component %v: %w", c.Name, err)
	}
	qpath := filepath.Join(r.QuadletPrefix, c.Name)
	err = os.Mkdir(qpath, 0o755)
	if err != nil {
		return fmt.Errorf("error installing component: %w", err)
	}

	qFile, err := os.OpenFile(fmt.Sprintf("%v/.materia_managed", qpath), os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("error installing component: %w", err)
	}
	defer func() { _ = qFile.Close() }()
	vd, err := c.VersonData()
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(r.DataPrefix, c.Name, ".component_version"), vd.Bytes(), 0o755)
	if err != nil {
		return err
	}
	return nil
}

func (r *HostComponentRepository) RemoveComponent(c *components.Component) error {
	if c == nil {
		return errors.New("invalid component")
	}
	compName := c.Name
	entries, err := os.ReadDir(filepath.Join(r.DataPrefix, compName))
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		// TODO make prettier
		if len(entries) != 1 || entries[0].Name() != ".component_version" {
			return errors.New("component data folder not empty")
		} else {
			err = os.Remove(filepath.Join(r.DataPrefix, compName, ".component_version"))
			if err != nil {
				return err
			}
		}
	}
	err = os.Remove(filepath.Join(r.DataPrefix, compName))
	if err != nil {
		return err
	}

	err = os.Remove(filepath.Join(r.QuadletPrefix, compName, ".materia_managed"))
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(r.QuadletPrefix, compName))
	return err
}

func (r *HostComponentRepository) ReadResource(res components.Resource) (string, error) {
	resPath := ""
	if isQuadlet(res) {
		resPath = filepath.Join(r.QuadletPrefix, res.Parent, res.Path)
	} else {
		resPath = filepath.Join(r.DataPrefix, res.Parent, res.Path)
	}

	curFile, err := os.ReadFile(resPath)
	if err != nil {
		return "", err
	}
	return string(curFile), nil
}

func (r *HostComponentRepository) InstallResource(res components.Resource, data *bytes.Buffer) error {
	if err := res.Validate(); err != nil {
		return fmt.Errorf("can't install invalid resource: %w", err)
	}
	if isQuadlet(res) {
		return os.WriteFile(filepath.Join(r.QuadletPrefix, res.Parent, res.Name), data.Bytes(), 0o755)
	}
	// TODO probably doing something stupid here
	prefix := filepath.Join(r.DataPrefix, res.Parent)
	parent := filepath.Dir(res.Path)
	if parent != "/" {
		fmt.Fprintf(os.Stderr, "FBLTHP[212]: newcomponent.go:358: parent=%+v\n", parent)
		parentPath := filepath.Join(prefix, parent)
		err := os.MkdirAll(parentPath, 0o755)
		if err != nil {
			return err
		}
	}
	resPath := filepath.Join(prefix, parent, res.Name)
	fmt.Fprintf(os.Stderr, "FBLTHP[211]: newcomponent.go:366: resPath=%+v\n", resPath)
	err := os.WriteFile(resPath, data.Bytes(), 0o755)
	return err
}

func (r *HostComponentRepository) RemoveResource(res components.Resource) error {
	resPath := ""
	if isQuadlet(res) {
		resPath = filepath.Join(r.QuadletPrefix, res.Parent, res.Path)
	} else {
		resPath = filepath.Join(r.DataPrefix, res.Parent, res.Path)
	}
	return os.Remove(resPath)
}

func (r *HostComponentRepository) ComponentExists(name string) (bool, error) {
	path := filepath.Join(r.DataPrefix, name)
	fmt.Fprintf(os.Stderr, "FBLTHP[219]: component.go:389: path=%+v\n", path)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *HostComponentRepository) PurgeComponent(c *components.Component) error {
	if c == nil {
		return errors.New("no component specified")
	}
	compName := c.Name
	err := os.RemoveAll(filepath.Join(r.DataPrefix, compName))
	if err != nil {
		return err
	}

	err = os.RemoveAll(filepath.Join(r.QuadletPrefix, compName))
	return err
}

func (r *HostComponentRepository) PurgeComponentByName(name string) error {
	if name == "" {
		return errors.New("no component specified")
	}
	err := os.RemoveAll(filepath.Join(r.DataPrefix, name))
	if err != nil {
		return err
	}

	err = os.RemoveAll(filepath.Join(r.QuadletPrefix, name))
	return err
}

func (r *HostComponentRepository) Clean() error {
	entries, err := os.ReadDir(r.QuadletPrefix)
	if err != nil {
		return err
	}
	for _, v := range entries {
		_, err := os.Stat(fmt.Sprintf("%v/%v/.materia_managed", r.QuadletPrefix, v.Name()))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		err = os.RemoveAll(filepath.Join(r.QuadletPrefix, v.Name()))
		if err != nil {
			return err
		}

	}
	return nil
}

func (r *HostComponentRepository) RunCleanup(comp *components.Component) error {
	path := filepath.Join(r.DataPrefix, comp.Name)
	cmd := exec.Command(fmt.Sprintf("%v/cleanup.sh", path))

	cmd.Dir = path
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func (r HostComponentRepository) RunSetup(comp *components.Component) error {
	path := filepath.Join(r.DataPrefix, comp.Name)
	cmd := exec.Command(fmt.Sprintf("%v/setup.sh", path))

	cmd.Dir = path
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func isQuadlet(res components.Resource) bool {
	switch res.Kind {
	case components.ResourceTypeContainer, components.ResourceTypeKube, components.ResourceTypeVolume, components.ResourceTypeNetwork, components.ResourceTypePod:
		return true
	default:
		return false
	}
}
