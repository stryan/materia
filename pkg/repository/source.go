package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/log/v2"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

var (
	ErrNeedHostRepository = errors.New("action can't be done on source repository")
	ErrNoSymlink          = errors.New("symlinks are not allowed")
)

type SourceComponentRepository struct {
	srcCompDir string // /var/lib/materia/source
	registry   *RemoteComponentRegistry
}

func NewSourceComponentRepository(sourceDir string, registry *RemoteComponentRegistry) (*SourceComponentRepository, error) {
	if _, err := os.Stat(sourceDir); err != nil {
		// we expect the source base dir to be pre-created for us
		return nil, err
	}
	if registry == nil {
		var err error
		registry, err = NewRemoteComponentRegistry("")
		if err != nil {
			return nil, err
		}
	}

	return &SourceComponentRepository{
		srcCompDir: sourceDir,
		registry:   registry,
	}, nil
}

func (s *SourceComponentRepository) getPrefix(name string) (string, error) {
	name, _, _ = strings.Cut(name, "@") // source components can't be instanced anyway
	if err := components.ValidateComponentName(name); err != nil {
		return "", err
	}

	// check source root first
	sourceLocation := filepath.Join(s.srcCompDir, "components", name)
	info, err := lstatSource(sourceLocation)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("stray resource in source repo: %v", name)
		}
		return sourceLocation, nil
	}

	// check registry
	if path, found, err := s.registry.Resolve(name); err != nil {
		return "", err
	} else if found {
		return path, nil
	}

	return "", fmt.Errorf("component %q: %w", name, fs.ErrNotExist)
}

func (s SourceComponentRepository) Validate() error {
	if s.srcCompDir == "" && s.registry.Size() == 0 {
		return errors.New("no search paths for source components")
	}
	return nil
}

func (s *SourceComponentRepository) ReadResource(res components.Resource) (string, error) {
	if res.Kind == components.ResourceTypeDirectory || res.Kind == components.ResourceTypeDropinDir {
		return "", nil
	}
	prefix, err := s.getPrefix(res.Parent)
	if err != nil {
		return "", err
	}
	root, err := os.OpenRoot(prefix)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()

	curFile, err := root.ReadFile(res.Filepath())
	if err != nil {
		return "", err
	}
	return string(curFile), nil
}

func (s *SourceComponentRepository) ListComponentNames() ([]string, error) {
	var compPaths []string

	entries, err := os.ReadDir(filepath.Join(s.srcCompDir, "components"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, v := range entries {
		if strings.HasPrefix(v.Name(), ".") {
			continue
		}
		if v.IsDir() || v.Type()&fs.ModeSymlink != 0 {
			compPaths = append(compPaths, v.Name())
		}
	}
	compPaths = append(compPaths, s.registry.List()...)
	slices.Sort(compPaths)
	compPaths = slices.Compact(compPaths)
	return compPaths, nil
}

func (s *SourceComponentRepository) Clean() error {
	cerr := os.RemoveAll(s.srcCompDir)
	rerr := s.registry.Clean()
	return errors.Join(cerr, rerr)
}

func (s *SourceComponentRepository) GetComponent(name string) (*components.Component, error) {
	if strings.Contains(name, "@") {
		name = strings.Split(name, "@")[0]
	}
	path, err := s.getPrefix(name)
	if err != nil {
		return nil, err
	}
	c, err := components.NewComponent(name)
	if err != nil {
		return nil, err
	}
	c.State = components.StateFresh
	c.Version = components.DefaultComponentVersion
	log.Debugf("loading source component %v from path %v", c.Name, path)

	resources, err := s.loadResources(c, path)
	if err != nil {
		return nil, err
	}
	for _, r := range resources {
		err := c.Resources.Add(r)
		if err != nil {
			return nil, err
		}
	}
	if !c.Resources.Contains(manifests.ComponentManifestFile) {
		return nil, components.ErrCorruptComponent
	}
	// secrets are added on Manifest application, so we're done here

	return c, nil
}

func (s *SourceComponentRepository) GetResource(parent *components.Component, name string) (components.Resource, error) {
	if parent == nil || name == "" || !filepath.IsLocal(name) {
		return components.Resource{}, errors.New("invalid parent or resource")
	}
	prefix, err := s.getPrefix(parent.Name)
	if err != nil {
		return components.Resource{}, err
	}
	for _, candidate := range []string{name, name + ".gotmpl"} {
		p := filepath.Join(prefix, candidate)
		if _, err := os.Lstat(p); err == nil {
			return s.NewResource(parent, p)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return components.Resource{}, err
		}
	}
	return components.Resource{}, fmt.Errorf("resource %q: %w", name, fs.ErrNotExist)
}

func (s *SourceComponentRepository) ListResources(c *components.Component) ([]components.Resource, error) {
	if c == nil {
		return []components.Resource{}, errors.New("invalid parent or resource")
	}
	dataPath, err := s.getPrefix(c.Name)
	if err != nil {
		return nil, err
	}
	return s.loadResources(c, dataPath)
}

func (s *SourceComponentRepository) ComponentExists(name string) (bool, error) {
	_, err := s.getPrefix(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (s *SourceComponentRepository) GetManifest(parent *components.Component) (*manifests.ComponentManifest, error) {
	prefix, err := s.getPrefix(parent.Name)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(prefix)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	mani, err := root.ReadFile(manifests.ComponentManifestFile)
	if err != nil {
		return nil, err
	}
	// make sure it's not a symlink
	if _, err := lstatSource(filepath.Join(prefix, manifests.ComponentManifestFile)); err != nil {
		return nil, err
	}
	return manifests.LoadComponentManifestFromContent(mani)
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
	fileInfo, err := lstatSource(path)
	if err != nil {
		return components.Resource{}, err
	}
	res := components.Resource{
		Path:     resName,
		Mode:     fileInfo.Mode().Perm(),
		Parent:   parent.Name,
		Template: components.IsTemplate(path),
	}
	if fileInfo.IsDir() {
		if components.IsDropinDir(resName) {
			res.Kind = components.ResourceTypeDropinDir
		} else {
			res.Kind = components.ResourceTypeDirectory
		}
	} else {
		res.Kind = components.FindResourceType(resName)
	}
	return res, nil
}

func (s *SourceComponentRepository) loadResources(c *components.Component, path string) ([]components.Resource, error) {
	var out []components.Resource
	err := filepath.WalkDir(path, func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if fullPath == path {
			return nil
		}
		if d.Name() == ".git" {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		res, err := s.NewResource(c, fullPath)
		if err != nil {
			return err
		}
		out = append(out, res)
		return nil
	})
	return out, err
}

func lstatSource(path string) (fs.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		return nil, fmt.Errorf("invalid path %v: %w", path, ErrNoSymlink)
	case !info.Mode().IsRegular() && !info.IsDir():
		return nil, fmt.Errorf("unsupported file type %v: %v", info.Mode().Type(), path)
	}
	return info, nil
}
