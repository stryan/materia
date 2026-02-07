package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/charmbracelet/log"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

type HostComponentRepository struct {
	DataPrefix    string
	QuadletPrefix string
}

func NewHostComponentRepository(quadletPrefix, dataPrefix string) (*HostComponentRepository, error) {
	if _, err := os.Stat(dataPrefix); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.Mkdir(dataPrefix, 0o755)
			if err != nil {
				return nil, fmt.Errorf("error creating ComponentRepository with data_prefix %v: %w", dataPrefix, err)
			}
		}
	}
	if _, err := os.Stat(quadletPrefix); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.Mkdir(quadletPrefix, 0o755)
			if err != nil {
				return nil, fmt.Errorf("error creating FileRepository with quadlet prefix %v: %w", quadletPrefix, err)
			}
		}
	}
	return &HostComponentRepository{
		DataPrefix:    dataPrefix,
		QuadletPrefix: quadletPrefix,
	}, nil
}

func (r *HostComponentRepository) GetComponent(name string) (*components.Component, error) {
	oldComp := components.NewComponent(name)
	dataPath := filepath.Join(r.DataPrefix, name)
	quadletPath := filepath.Join(r.QuadletPrefix, name)
	// load resources
	versionFileExists := true
	_, err := os.Stat(filepath.Join(dataPath, ".component_version"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			versionFileExists = false
		} else {
			return nil, fmt.Errorf("error reading component version: %w", err)
		}
	}
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
	log.Debug("loading component", "component", oldComp.Name, "version", oldComp.Version)
	manifestPath := filepath.Join(dataPath, manifests.ComponentManifestFile)
	if _, err := os.Stat(manifestPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, components.ErrCorruptComponent
		}
		return nil, err
	}
	manifestResource, err := r.NewResource(oldComp, manifestPath)
	if err != nil {
		return nil, err
	}
	err = oldComp.Resources.Add(manifestResource)
	if err != nil {
		return nil, err
	}

	err = filepath.WalkDir(dataPath, func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == oldComp.Name || d.Name() == ".component_version" || d.Name() == manifests.ComponentManifestFile {
			return nil
		}
		newRes, err := r.NewResource(oldComp, fullPath)
		if err != nil {
			return err
		}
		err = oldComp.Resources.Add(newRes)
		if err != nil {
			return err
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

		newRes, err := r.NewResource(oldComp, fullPath)
		if err != nil {
			return err
		}

		return oldComp.Resources.Add(newRes)
	})
	if err != nil {
		return nil, err
	}

	return oldComp, nil
}

func (r *HostComponentRepository) GetManifest(parent *components.Component) (*manifests.ComponentManifest, error) {
	return manifests.LoadComponentManifestFromFile(filepath.Join(r.DataPrefix, parent.Name, manifests.ComponentManifestFile))
}

func (r *HostComponentRepository) GetResource(parent *components.Component, name string) (components.Resource, error) {
	if parent == nil || name == "" {
		return components.Resource{}, errors.New("invalid parent or resource")
	}
	dataPath := filepath.Join(r.DataPrefix, parent.Name)
	quadletPath := filepath.Join(r.QuadletPrefix, parent.Name)
	resourcePath := filepath.Join(dataPath, name)
	_, err := os.Stat(resourcePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return components.Resource{}, err
	} else if err != nil {
		return r.NewResource(parent, resourcePath)
	}

	resourcePath = filepath.Join(quadletPath, name)
	_, err = os.Stat(resourcePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return components.Resource{}, err
	} else if err != nil {
		return r.NewResource(parent, resourcePath)
	}
	return components.Resource{}, errors.New("resource not found")
}

func (r *HostComponentRepository) ListResources(c *components.Component) ([]components.Resource, error) {
	if c == nil {
		return []components.Resource{}, errors.New("invalid parent or resource")
	}
	resources := []components.Resource{}
	dataPath := filepath.Join(r.DataPrefix, c.Name)
	quadletPath := filepath.Join(r.QuadletPrefix, c.Name)
	searchFunc := func(prefix string) fs.WalkDirFunc {
		return func(fullPath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.Name() == c.Name || d.Name() == ".component_version" || d.Name() == ".materia_managed" {
				return nil
			}
			resName, err := filepath.Rel(prefix, fullPath)
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
	}
	err := filepath.WalkDir(dataPath, searchFunc(dataPath))
	if err != nil {
		return resources, err
	}
	err = filepath.WalkDir(quadletPath, searchFunc(quadletPath))
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
	slices.Sort(compPaths)
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

	qFile, err := os.OpenFile(fmt.Sprintf("%v/.materia_managed", qpath), os.O_RDONLY|os.O_CREATE, 0o666)
	if err != nil {
		return fmt.Errorf("error installing component: %w", err)
	}
	defer func() {
		closeErr := qFile.Close()
		if closeErr != nil {
			if err == nil {
				err = closeErr
			} else {
				log.Warnf("can't close file while installing component %v: %v", c.Name, closeErr)
			}
		}
	}()
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

func (r *HostComponentRepository) UpdateComponent(c *components.Component) error {
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
	err := os.Remove(filepath.Join(r.DataPrefix, compName, ".component_version"))
	if err != nil {
		return err
	}
	leftovers := []string{}
	err = filepath.WalkDir(filepath.Join(r.DataPrefix, compName), func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return fmt.Errorf("component data folder not empty: %v", d.Name())
		}
		leftovers = append(leftovers, fullPath)
		return nil
	})
	if err != nil {
		return err
	}
	for _, leftoverDir := range leftovers {
		err = os.Remove(leftoverDir)
		if err != nil {
			return err
		}
	}
	err = os.Remove(filepath.Join(r.QuadletPrefix, compName, ".materia_managed"))
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(r.QuadletPrefix, compName))
	return err
}

func (r *HostComponentRepository) NewResource(parent *components.Component, path string) (components.Resource, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return components.Resource{}, err
	}
	res := components.Resource{
		Path:     path,
		Parent:   parent.Name,
		Template: false,
	}
	if fileInfo.IsDir() {
		res.Kind = components.ResourceTypeDirectory
	} else {
		res.Kind = components.FindResourceType(path)
	}
	if res.IsQuadlet() {
		res.Path, err = filepath.Rel(filepath.Join(r.QuadletPrefix, parent.Name), path)
		if err != nil {
			return res, err
		}
		unitData, err := r.ReadResource(res)
		if err != nil {
			return res, err
		}
		hostObject, err := res.GetHostObject(unitData)
		if err != nil {
			return res, err
		}
		res.HostObject = hostObject
	} else {

		res.Path, err = filepath.Rel(filepath.Join(r.DataPrefix, parent.Name), path)
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

func (r *HostComponentRepository) ReadResource(res components.Resource) (string, error) {
	resPath := ""
	if res.Kind == components.ResourceTypeDirectory {
		return "", nil
	}
	if res.Kind == components.ResourceTypePodmanSecret {
		return "", errors.New("secrets don't live in repositories")
	}
	if res.IsQuadlet() {
		resPath = filepath.Join(r.QuadletPrefix, res.Parent, res.Filepath())
	} else {
		resPath = filepath.Join(r.DataPrefix, res.Parent, res.Filepath())
	}

	curFile, err := os.ReadFile(resPath)
	if err != nil {
		return "", err
	}
	return string(curFile), nil
}

func (r *HostComponentRepository) InstallResource(res components.Resource, data []byte) error {
	if err := res.Validate(); err != nil {
		return fmt.Errorf("can't install invalid resource %v: %w", res.Path, err)
	}
	if res.IsQuadlet() {
		return os.WriteFile(filepath.Join(r.QuadletPrefix, res.Parent, res.Path), data, 0o755)
	}
	// TODO probably doing something stupid here
	prefix := filepath.Join(r.DataPrefix, res.Parent)
	if res.Kind == components.ResourceTypeDirectory {
		err := os.Mkdir(filepath.Join(prefix, res.Path), 0o755)
		if err != nil {
			return err
		}
		return nil
	}
	resPath := filepath.Join(prefix, res.Path)
	err := os.WriteFile(resPath, data, 0o755)
	return err
}

func (r *HostComponentRepository) RemoveResource(res components.Resource) error {
	resPath := ""
	if res.IsQuadlet() {
		resPath = filepath.Join(r.QuadletPrefix, res.Parent, res.Path)
	} else {
		resPath = filepath.Join(r.DataPrefix, res.Parent, res.Path)
	}
	err := os.Remove(resPath)
	if err != nil {
		return err
	}
	return nil
}

func (r *HostComponentRepository) ComponentExists(name string) (bool, error) {
	path := filepath.Join(r.DataPrefix, name)
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
		if !v.IsDir() {
			continue
		}
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
	return os.RemoveAll(r.DataPrefix)
}
