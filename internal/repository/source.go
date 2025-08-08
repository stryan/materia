package repository

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/manifests"
)

var ErrNeedHostRepository = errors.New("action can't be done on source repository")

type SourceComponentRepository struct {
	Prefix string
}

func NewSourceComponentRepository(dataPrefix string) (*SourceComponentRepository, error) {
	if _, err := os.Stat(dataPrefix); err != nil {
		// we expect the source repo to be pre-created for us
		return nil, err
	}
	return &SourceComponentRepository{
		Prefix: dataPrefix,
	}, nil
}

func (s SourceComponentRepository) Validate() error {
	if s.Prefix == "" {
		return errors.New("no data prefix")
	}
	return nil
}

func (s *SourceComponentRepository) ReadResource(res components.Resource) (string, error) {
	if res.Kind == components.ResourceTypeDirectory {
		return "", nil
	}
	resPath := filepath.Join(s.Prefix, res.Parent, res.Path)

	curFile, err := os.ReadFile(resPath)
	if err != nil {
		return "", err
	}
	return string(curFile), nil
}

func (s *SourceComponentRepository) ListComponentNames() ([]string, error) {
	var compPaths []string
	entries, err := os.ReadDir(s.Prefix)
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

func (s *SourceComponentRepository) Clean() error {
	entries, err := os.ReadDir(s.Prefix)
	if err != nil {
		return err
	}
	for _, v := range entries {
		err = os.RemoveAll(filepath.Join(s.Prefix, v.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SourceComponentRepository) GetComponent(name string) (*components.Component, error) {
	path := filepath.Join(s.Prefix, name)
	c := &components.Component{}
	c.Name = name
	c.State = components.StateFresh
	c.Defaults = make(map[string]any)
	c.Version = components.DefaultComponentVersion
	c.VolumeResources = make(map[string]manifests.VolumeResourceConfig)
	c.ServiceResources = make(map[string]manifests.ServiceResourceConfig)
	log.Debugf("loading source component %v from path %v", c.Name, path)
	var man *manifests.ComponentManifest
	scripts := 0
	err := filepath.WalkDir(path, func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == c.Name {
			return nil
		}
		resPath := strings.TrimPrefix(fullPath, path)
		fmt.Fprintf(os.Stderr, "FBLTHP[299]: source.go:102: resPath=%+v\n", resPath)
		if d.Name() == "MANIFEST.toml" {
			log.Debugf("loading source component manifest %v", c.Name)
			man, err = manifests.LoadComponentManifest(fullPath)
			if err != nil {
				return fmt.Errorf("error loading component manifest: %w", err)
			}
			maps.Copy(c.Defaults, man.Defaults)
			maps.Copy(c.VolumeResources, man.VolumeResources)
			return nil
		}
		var newRes components.Resource
		if d.Name() == "setup.sh" || d.Name() == "cleanup.sh" {
			scripts++
			c.Scripted = true
			newRes = components.Resource{
				Path:     resPath,
				Name:     d.Name(),
				Parent:   c.Name,
				Kind:     components.ResourceTypeComponentScript,
				Template: false,
			}
		} else {
			newRes = components.Resource{
				Path:     resPath,
				Parent:   c.Name,
				Name:     strings.TrimSuffix(d.Name(), ".gotmpl"),
				Kind:     components.FindResourceType(d.Name()),
				Template: components.IsTemplate(d.Name()),
			}
			if d.IsDir() {
				newRes.Kind = components.ResourceTypeDirectory
				newRes.Template = false
			}
		}
		for _, vr := range c.VolumeResources {
			if vr.Resource == newRes.Name {
				newRes.Kind = components.ResourceTypeVolumeFile
			}
		}
		c.Resources = append(c.Resources, newRes)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if man == nil {
		return nil, components.ErrCorruptComponent
	}
	if scripts != 0 && scripts != 2 {
		return nil, errors.New("scripted component is missing install or cleanup")
	}
	for _, s := range man.Services {
		if err := s.Validate(); err != nil {
			return nil, fmt.Errorf("invalid service for component: %w", err)
		}
		c.ServiceResources[s.Service] = s
	}

	c.Resources = append(c.Resources, components.Resource{
		Parent:   c.Name,
		Path:     "/MANIFEST.toml",
		Name:     "MANIFEST.toml",
		Kind:     components.ResourceTypeManifest,
		Template: false,
	})
	for k, r := range c.Resources {
		if r.Kind != components.ResourceTypeScript && slices.Contains(man.Scripts, r.Name) {
			r.Kind = components.ResourceTypeScript
			c.Resources[k] = r
		}
	}

	return c, nil
}

func (r *SourceComponentRepository) GetResource(parent *components.Component, name string) (components.Resource, error) {
	if parent == nil || name == "" {
		return components.Resource{}, errors.New("invalid parent or resource")
	}
	dataPath := filepath.Join(r.Prefix, parent.Name)
	resourcePath := ""
	breakWalk := false
	searchFunc := func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if breakWalk {
			return nil
		}
		if d.Name() == parent.Name {
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

func (r *SourceComponentRepository) ListResources(c *components.Component) ([]components.Resource, error) {
	if c == nil {
		return []components.Resource{}, errors.New("invalid parent or resource")
	}
	resources := []components.Resource{}
	dataPath := filepath.Join(r.Prefix, c.Name)
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
	return resources, nil
}

func (r *SourceComponentRepository) InstallComponent(c *components.Component) error {
	return fmt.Errorf("can't install component: %w", ErrNeedHostRepository)
}

func (r *SourceComponentRepository) UpdateComponent(c *components.Component) error {
	return fmt.Errorf("can't update component: %w", ErrNeedHostRepository)
}

func (r *SourceComponentRepository) RemoveComponent(c *components.Component) error {
	return fmt.Errorf("can't remove component: %w", ErrNeedHostRepository)
}

func (r *SourceComponentRepository) InstallResource(res components.Resource, data *bytes.Buffer) error {
	return fmt.Errorf("can't install resource: %w", ErrNeedHostRepository)
}

func (r *SourceComponentRepository) RemoveResource(res components.Resource) error {
	return fmt.Errorf("can't remove resource: %w", ErrNeedHostRepository)
}

func (r *SourceComponentRepository) ComponentExists(name string) (bool, error) {
	_, err := os.Stat(filepath.Join(r.Prefix, name))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *SourceComponentRepository) PurgeComponent(c *components.Component) error {
	return fmt.Errorf("can't purge component: %w", ErrNeedHostRepository)
}

func (r *SourceComponentRepository) PurgeComponentByName(name string) error {
	return fmt.Errorf("can't purge component: %w", ErrNeedHostRepository)
}

func (r *SourceComponentRepository) RunCleanup(comp *components.Component) error {
	return fmt.Errorf("can't run cleanup script: %w", ErrNeedHostRepository)
}

func (r SourceComponentRepository) RunSetup(comp *components.Component) error {
	return fmt.Errorf("can't run setup script: %w", ErrNeedHostRepository)
}

func (s *SourceComponentRepository) GetManifest(parent *components.Component) (*manifests.ComponentManifest, error) {
	return manifests.LoadComponentManifest(filepath.Join(s.Prefix, parent.Name, "MANIFEST.toml"))
}
