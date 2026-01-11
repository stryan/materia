package repository

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/pkg/manifests"
)

var ErrNeedHostRepository = errors.New("action can't be done on source repository")

type SourceComponentRepository struct {
	basedirs []string
}

func NewSourceComponentRepository(sourceDirs ...string) (*SourceComponentRepository, error) {
	for _, sourceDir := range sourceDirs {
		if _, err := os.Stat(sourceDir); err != nil {
			// we expect the source repos to be pre-created for us
			return nil, err
		}
	}
	return &SourceComponentRepository{
		basedirs: sourceDirs,
	}, nil
}

func (s SourceComponentRepository) getPrefix(name string) (string, error) {
	for _, bd := range s.basedirs {
		if _, err := os.Stat(filepath.Join(bd, "components", name)); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return "", err
		} else {
			return filepath.Join(bd, "components", name), nil
		}
	}
	return "", fmt.Errorf("can't get prefix for resource %v", name)
}

func (s SourceComponentRepository) Validate() error {
	if len(s.basedirs) < 1 {
		return errors.New("no search paths for source components")
	}
	return nil
}

func (s *SourceComponentRepository) ReadResource(res components.Resource) (string, error) {
	if res.Kind == components.ResourceTypeDirectory {
		return "", nil
	}
	prefix, err := s.getPrefix(res.Parent)
	if err != nil {
		return "", err
	}
	resPath := filepath.Join(prefix, res.Filepath())

	curFile, err := os.ReadFile(resPath)
	if err != nil {
		return "", err
	}
	return string(curFile), nil
}

func (s *SourceComponentRepository) ListComponentNames() ([]string, error) {
	var compPaths []string
	for _, bd := range s.basedirs {
		entries, err := os.ReadDir(bd)
		if err != nil {
			return nil, err
		}
		for _, v := range entries {
			if v.IsDir() {
				compPaths = append(compPaths, v.Name())
			}
		}
	}
	slices.Sort(compPaths)
	return compPaths, nil
}

func (s *SourceComponentRepository) Clean() error {
	for _, bd := range s.basedirs {
		if err := os.RemoveAll(bd); err != nil {
			return err
		}
	}
	return nil
}

func (s *SourceComponentRepository) GetComponent(name string) (*components.Component, error) {
	path, err := s.getPrefix(name)
	if err != nil {
		return nil, err
	}
	c := components.NewComponent(name)
	c.State = components.StateFresh
	c.Version = components.DefaultComponentVersion
	log.Debugf("loading source component %v from path %v", c.Name, path)
	scripts := 0

	secretResources := []components.Resource{}
	manifestPath := filepath.Join(path, manifests.ComponentManifestFile)
	if _, err := os.Stat(manifestPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, components.ErrCorruptComponent
		}
		return nil, err
	}
	err = filepath.WalkDir(path, func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == c.Name || d.Name() == manifests.ComponentManifestFile {
			return nil
		}
		if strings.Contains(fullPath, ".git") {
			return nil
		}
		newRes, err := s.NewResource(c, fullPath)
		if err != nil {
			return err
		}
		if newRes.Kind == components.ResourceTypeComponentScript {
			scripts++
			c.Scripted = true
		}
		return c.Resources.Add(newRes)
	})
	if err != nil {
		return nil, err
	}
	if scripts != 0 && scripts != 2 {
		return nil, errors.New("scripted component is missing install or cleanup")
	}
	for _, s := range secretResources {
		err := c.Resources.Add(s)
		if err != nil {
			return nil, err
		}
	}
	manifestResource, err := s.NewResource(c, manifestPath)
	if err != nil {
		return nil, err
	}

	return c, c.Resources.Add(manifestResource)
}

func (s *SourceComponentRepository) GetResource(parent *components.Component, name string) (components.Resource, error) {
	if parent == nil || name == "" {
		return components.Resource{}, errors.New("invalid parent or resource")
	}
	prefix, err := s.getPrefix(parent.Name)
	if err != nil {
		return components.Resource{}, err
	}
	resourcePath := filepath.Join(prefix, name)
	return s.NewResource(parent, resourcePath)
}

func (s *SourceComponentRepository) ListResources(c *components.Component) ([]components.Resource, error) {
	if c == nil {
		return []components.Resource{}, errors.New("invalid parent or resource")
	}
	resources := []components.Resource{}
	dataPath, err := s.getPrefix(c.Name)
	if err != nil {
		return resources, err
	}
	searchFunc := func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Name() == c.Name || d.Name() == ".component_version" || d.Name() == ".materia_managed" {
			return nil
		}
		resName, err := filepath.Rel(dataPath, fullPath)
		if err != nil {
			return err
		}
		newRes := components.Resource{
			Parent:   c.Name,
			Path:     resName,
			Kind:     components.FindResourceType(resName),
			Template: components.IsTemplate(resName),
		}
		resources = append(resources, newRes)

		return nil
	}
	err = filepath.WalkDir(dataPath, searchFunc)
	if err != nil {
		return resources, err
	}
	return resources, nil
}

func (s *SourceComponentRepository) InstallComponent(c *components.Component) error {
	return fmt.Errorf("can't install component: %w", ErrNeedHostRepository)
}

func (s *SourceComponentRepository) UpdateComponent(c *components.Component) error {
	return fmt.Errorf("can't update component: %w", ErrNeedHostRepository)
}

func (s *SourceComponentRepository) RemoveComponent(c *components.Component) error {
	return fmt.Errorf("can't remove component: %w", ErrNeedHostRepository)
}

func (s *SourceComponentRepository) InstallResource(res components.Resource, data *bytes.Buffer) error {
	return fmt.Errorf("can't install resource: %w", ErrNeedHostRepository)
}

func (s *SourceComponentRepository) RemoveResource(res components.Resource) error {
	return fmt.Errorf("can't remove resource: %w", ErrNeedHostRepository)
}

func (s *SourceComponentRepository) ComponentExists(name string) (bool, error) {
	_, err := s.getPrefix(name)
	return (err == nil), err
}

func (s *SourceComponentRepository) PurgeComponent(c *components.Component) error {
	return fmt.Errorf("can't purge component: %w", ErrNeedHostRepository)
}

func (s *SourceComponentRepository) PurgeComponentByName(name string) error {
	return fmt.Errorf("can't purge component: %w", ErrNeedHostRepository)
}

func (s *SourceComponentRepository) RunCleanup(comp *components.Component) error {
	return fmt.Errorf("can't run cleanup script: %w", ErrNeedHostRepository)
}

func (s SourceComponentRepository) RunSetup(comp *components.Component) error {
	return fmt.Errorf("can't run setup script: %w", ErrNeedHostRepository)
}

func (s *SourceComponentRepository) GetManifest(parent *components.Component) (*manifests.ComponentManifest, error) {
	prefix, err := s.getPrefix(parent.Name)
	if err != nil {
		return nil, err
	}

	return manifests.LoadComponentManifest(filepath.Join(prefix, manifests.ComponentManifestFile))
}

func (s *SourceComponentRepository) NewResource(parent *components.Component, path string) (components.Resource, error) {
	filename := strings.TrimSuffix(path, ".gotmpl")
	prefix, err := s.getPrefix(parent.Name)
	if err != nil {
		return components.Resource{}, err
	}
	resName, err := filepath.Rel(prefix, filename)
	if err != nil {
		return components.Resource{}, err
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		return components.Resource{}, err
	}
	res := components.Resource{
		Path:     resName,
		Parent:   parent.Name,
		Template: components.IsTemplate(path),
	}
	if fileInfo.IsDir() {
		res.Kind = components.ResourceTypeDirectory
	} else {
		res.Kind = components.FindResourceType(path)
	}
	return res, nil
}
