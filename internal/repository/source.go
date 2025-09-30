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
	basedir string
	Prefix  string
}

func NewSourceComponentRepository(sourceDir string) (*SourceComponentRepository, error) {
	if _, err := os.Stat(sourceDir); err != nil {
		// we expect the source repo to be pre-created for us
		return nil, err
	}
	return &SourceComponentRepository{
		basedir: sourceDir,
		Prefix:  filepath.Join(sourceDir, "components"),
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
	resPath := filepath.Join(s.Prefix, res.Parent, res.Name())

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
	return os.RemoveAll(s.basedir)
}

func (s *SourceComponentRepository) GetComponent(name string) (*components.Component, error) {
	path := filepath.Join(s.Prefix, name)
	c := &components.Component{}
	c.Name = name
	c.State = components.StateFresh
	c.Defaults = make(map[string]any)
	c.Version = components.DefaultComponentVersion
	c.ServiceResources = make(map[string]manifests.ServiceResourceConfig)
	log.Debugf("loading source component %v from path %v", c.Name, path)
	var man *manifests.ComponentManifest
	scripts := 0

	secretResources := []components.Resource{}
	manifestPath := filepath.Join(path, manifests.ComponentManifestFile)
	if _, err := os.Stat(manifestPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, components.ErrCorruptComponent
		}
		return nil, err
	}
	log.Debugf("loading source component manifest %v", c.Name)
	man, err := manifests.LoadComponentManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("error loading component manifest: %w", err)
	}
	maps.Copy(c.Defaults, man.Defaults)
	slices.Sort(man.Secrets)
	for _, s := range man.Secrets {
		secretResources = append(secretResources, components.Resource{
			Path:     s,
			Kind:     components.ResourceTypePodmanSecret,
			Parent:   name,
			Template: false,
		})
	}

	err = filepath.WalkDir(path, func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == c.Name || d.Name() == manifests.ComponentManifestFile {
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
	c.Resources = append(c.Resources, secretResources...)
	manifestResource, err := s.NewResource(c, manifestPath)
	if err != nil {
		return nil, err
	}
	c.Resources = append(c.Resources, manifestResource)
	for k, r := range c.Resources {
		if r.Kind != components.ResourceTypeScript && slices.Contains(man.Scripts, r.Path) {
			r.Kind = components.ResourceTypeScript
			c.Resources[k] = r
		}
	}

	return c, nil
}

func (s *SourceComponentRepository) GetResource(parent *components.Component, name string) (components.Resource, error) {
	if parent == nil || name == "" {
		return components.Resource{}, errors.New("invalid parent or resource")
	}
	dataPath := filepath.Join(s.Prefix, parent.Name)
	resourcePath := filepath.Join(dataPath, name)
	return s.NewResource(parent, resourcePath)
}

func (s *SourceComponentRepository) ListResources(c *components.Component) ([]components.Resource, error) {
	if c == nil {
		return []components.Resource{}, errors.New("invalid parent or resource")
	}
	resources := []components.Resource{}
	dataPath := filepath.Join(s.Prefix, c.Name)
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
	err := filepath.WalkDir(dataPath, searchFunc)
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
	_, err := os.Stat(filepath.Join(s.Prefix, name))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
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
	return manifests.LoadComponentManifest(filepath.Join(s.Prefix, parent.Name, manifests.ComponentManifestFile))
}

func (s *SourceComponentRepository) NewResource(parent *components.Component, path string) (components.Resource, error) {
	filename := strings.TrimSuffix(path, ".gotmpl")
	parentPath := filepath.Join(s.Prefix, parent.Name)
	resName, err := filepath.Rel(parentPath, filename)
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

	// TODO is this something we can do at source level? we can't parse unit files until they're templated
	// if res.IsQuadlet() {
	// 	unitData, err := s.ReadResource(res)
	// 	if err != nil {
	// 		return res, err
	// 	}
	// 	unitfile := parser.NewUnitFile()
	// 	err = unitfile.Parse(unitData)
	// 	if err != nil {
	// 		return res, fmt.Errorf("error parsing container file: %w", err)
	// 	}
	// 	nameOption := ""
	// 	group := ""
	// 	switch res.Kind {
	// 	case components.ResourceTypeContainer:
	// 		group = "Container"
	// 		nameOption = "ContainerName"
	// 	case components.ResourceTypeVolume:
	// 		group = "Volume"
	// 		nameOption = "VolumeName"
	// 	case components.ResourceTypeNetwork:
	// 		group = "Network"
	// 		nameOption = "NetworkName"
	// 	case components.ResourceTypePod:
	// 		group = "Pod"
	// 		nameOption = "PodName"
	// 	}
	// 	if nameOption != "" {
	// 		name, foundName := unitfile.Lookup(group, nameOption)
	// 		if foundName {
	// 			res.PodmanObject = name
	// 		} else {
	// 			res.PodmanObject = fmt.Sprintf("systemd-%v", strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	// 		}
	// 	}
	//
	// }
	return res, nil
}
