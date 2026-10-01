package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"charm.land/log/v2"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

type HostComponentRepository struct {
	dataPrefix    string
	quadletPrefix string
}

func NewHostComponentRepository(quadletPrefix, dataPrefix string) (*HostComponentRepository, error) {
	qp, err := filepath.Abs(quadletPrefix)
	if err != nil {
		return nil, err
	}
	dap, err := filepath.Abs(dataPrefix)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(dap, 0o755)
	if err != nil {
		return nil, fmt.Errorf("error creating ComponentRepository with data_prefix %v/%v: %w", dataPrefix, dap, err)
	}

	err = os.MkdirAll(qp, 0o755)
	if err != nil {
		return nil, fmt.Errorf("error creating ComponentRepository with quadletPrefix %v/%v: %w", quadletPrefix, qp, err)
	}
	return &HostComponentRepository{
		dataPrefix:    dap,
		quadletPrefix: qp,
	}, nil
}

func (r *HostComponentRepository) dRoot() (*os.Root, error) {
	return os.OpenRoot(r.dataPrefix)
}

func (r *HostComponentRepository) qRoot() (*os.Root, error) {
	return os.OpenRoot(r.quadletPrefix)
}

func openComp(prefix string, c *components.Component) (*os.Root, error) {
	parent, err := os.OpenRoot(prefix)
	if err != nil {
		return nil, err
	}
	defer func() { _ = parent.Close() }()
	root, err := parent.OpenRoot(c.InstanceName())
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %w", components.ErrCorruptComponent, err)
	}
	return root, err
}

func openRes(prefix string, res components.Resource) (*os.Root, error) {
	return os.OpenRoot(filepath.Join(prefix, res.Parent))
}

func (r *HostComponentRepository) GetComponent(name string) (*components.Component, error) {
	oldComp, err := components.NewComponent(name)
	if err != nil {
		return nil, err
	}
	dpath, err := openComp(r.dataPrefix, oldComp)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dpath.Close() }()
	qpath, err := openComp(r.quadletPrefix, oldComp)
	if err != nil {
		return nil, err
	}
	defer func() { _ = qpath.Close() }()
	// load resources
	versionFileExists := true
	_, err = dpath.Stat(".component_version")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			versionFileExists = false
		} else {
			return nil, fmt.Errorf("error reading component version: %w", err)
		}
	}
	if versionFileExists {
		k := koanf.New(".")
		filebytes, err := dpath.ReadFile(".component_version")
		if err != nil {
			return nil, fmt.Errorf("error reading component version content: %w", err)
		}
		err = k.Load(rawbytes.Provider(filebytes), toml.Parser())
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
	log.Debug("loading component", "component", oldComp.InstanceName(), "version", oldComp.Version)
	if _, err := dpath.Stat(manifests.ComponentManifestFile); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, components.ErrCorruptComponent
		}
		return nil, err
	}
	manifestResource, err := r.newResource(oldComp, dpath, false, manifests.ComponentManifestFile)
	if err != nil {
		return nil, err
	}
	err = oldComp.Resources.Add(manifestResource)
	if err != nil {
		return nil, err
	}
	err = fs.WalkDir(dpath.FS(), ".", func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if fullPath == "." || fullPath == ".component_version" || fullPath == manifests.ComponentManifestFile || tmpFileRegex.MatchString(d.Name()) {
			return nil
		}
		newRes, err := r.newResource(oldComp, dpath, false, fullPath)
		if err != nil {
			return err
		}
		return oldComp.Resources.Add(newRes)
	})
	if err != nil {
		return nil, err
	}
	err = fs.WalkDir(qpath.FS(), ".", func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == "." || d.Name() == ".materia_managed" {
			return nil
		}

		newRes, err := r.newResource(oldComp, qpath, true, fullPath)
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
	return manifests.LoadComponentManifestFromFile(filepath.Join(r.dataPrefix, parent.InstanceName(), manifests.ComponentManifestFile))
}

func (r *HostComponentRepository) GetResource(parent *components.Component, name string) (components.Resource, error) {
	if parent == nil || name == "" {
		return components.Resource{}, errors.New("invalid parent or resource")
	}

	dpath, err := openComp(r.dataPrefix, parent)
	if err != nil {
		return components.Resource{}, err
	}
	defer func() { _ = dpath.Close() }()
	qpath, err := openComp(r.quadletPrefix, parent)
	if err != nil {
		return components.Resource{}, err
	}
	defer func() { _ = qpath.Close() }()

	for _, t := range []struct {
		root   *os.Root
		isQuad bool
	}{{dpath, false}, {qpath, true}} {
		if _, err := t.root.Stat(name); err == nil {
			return r.newResource(parent, t.root, t.isQuad, name)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return components.Resource{}, err
		}
	}
	return components.Resource{}, errors.New("resource not found")
}

func (r *HostComponentRepository) ListResources(c *components.Component) ([]components.Resource, error) {
	if c == nil {
		return []components.Resource{}, errors.New("invalid parent or resource")
	}
	resources := []components.Resource{}
	dpath, err := openComp(r.dataPrefix, c)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dpath.Close() }()
	qpath, err := openComp(r.quadletPrefix, c)
	if err != nil {
		return nil, err
	}
	defer func() { _ = qpath.Close() }()
	searchFunc := func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." || p == ".component_version" || p == ".materia_managed" || tmpFileRegex.MatchString(p) {
			return nil
		}
		resources = append(resources, components.Resource{
			Parent:   c.InstanceName(),
			Path:     p,
			Kind:     components.FindResourceType(p),
			Template: components.IsTemplate(p),
		})
		return nil
	}
	err = fs.WalkDir(dpath.FS(), ".", searchFunc)
	if err != nil {
		return resources, err
	}
	err = fs.WalkDir(qpath.FS(), ".", searchFunc)
	if err != nil {
		return resources, err
	}
	return resources, nil
}

func (r *HostComponentRepository) ListComponentNames() ([]string, error) {
	var compPaths []string
	entries, err := os.ReadDir(r.dataPrefix)
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

func (r *HostComponentRepository) InstallComponent(c *components.Component) (err error) {
	if c == nil {
		return errors.New("nil component")
	}
	if err := c.Validate(); err != nil {
		return err
	}
	vd, err := c.VersionData()
	if err != nil {
		return err
	}
	droot, err := r.dRoot()
	if err != nil {
		return err
	}
	defer func() { _ = droot.Close() }()

	qroot, err := r.qRoot()
	if err != nil {
		return err
	}
	defer func() { _ = qroot.Close() }()
	if err := droot.Mkdir(c.InstanceName(), 0o755); err != nil {
		return fmt.Errorf("error installing component %v: %w", c.InstanceName(), err)
	}
	defer func() {
		if err != nil {
			_ = droot.RemoveAll(c.InstanceName())
		}
	}()
	if err := qroot.Mkdir(c.InstanceName(), 0o755); err != nil {
		return fmt.Errorf("error installing component %v: %w", c.InstanceName(), err)
	}
	defer func() {
		if err != nil {
			_ = qroot.RemoveAll(c.InstanceName())
		}
	}()

	if err := qroot.WriteFile(filepath.Join(c.InstanceName(), ".materia_managed"), nil, 0o644); err != nil {
		return fmt.Errorf("error installing component %v: %w", c.InstanceName(), err)
	}
	// Version file last, so a half-installed component never looks installed.
	if err := atomicWrite(droot, filepath.Join(c.InstanceName(), ".component_version"), 0o644, vd.Bytes()); err != nil {
		return fmt.Errorf("error installing component %v: %w", c.InstanceName(), err)
	}
	return nil
}

func (r *HostComponentRepository) UpdateComponent(c *components.Component) error {
	if c == nil {
		return errors.New("invalid component")
	}
	vd, err := c.VersionData()
	if err != nil {
		return err
	}
	dpath, err := openComp(r.dataPrefix, c)
	if err != nil {
		return err
	}
	defer func() { _ = dpath.Close() }()
	err = atomicWrite(dpath, ".component_version", 0o644, vd.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (r *HostComponentRepository) RemoveComponent(c *components.Component) error {
	if c == nil {
		return errors.New("invalid component")
	}
	dpath, err := openComp(r.dataPrefix, c)
	if err != nil {
		return err
	}
	defer func() { _ = dpath.Close() }()
	qpath, err := openComp(r.quadletPrefix, c)
	if err != nil {
		return err
	}
	defer func() { _ = qpath.Close() }()

	leftovers := []string{}
	err = fs.WalkDir(dpath.FS(), ".", func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == "." || d.Name() == ".component_version" || tmpFileRegex.MatchString(d.Name()) {
			return nil
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
	slices.Reverse(leftovers)
	for _, leftoverDir := range leftovers {
		err = dpath.Remove(leftoverDir)
		if err != nil {
			return err
		}
	}
	err = dpath.Remove(".component_version")
	if err != nil {
		return err
	}
	droot, err := r.dRoot()
	if err != nil {
		return err
	}
	defer func() { _ = droot.Close() }()

	qroot, err := r.qRoot()
	if err != nil {
		return err
	}
	defer func() { _ = qroot.Close() }()
	err = droot.Remove(c.InstanceName())
	if err != nil {
		return err
	}
	err = qpath.Remove(".materia_managed")
	if err != nil {
		return err
	}
	return qroot.Remove(c.InstanceName())
}

func (r *HostComponentRepository) newResource(parent *components.Component, root *os.Root, isQuadlet bool, path string) (components.Resource, error) {
	info, err := root.Stat(path)
	if err != nil {
		return components.Resource{}, err
	}
	rt := components.FindResourceType(path)
	res := components.Resource{Kind: rt, Path: path, Parent: parent.InstanceName()}
	if info.IsDir() {
		if isQuadlet && components.IsDropinDir(path) {
			res.Kind = components.ResourceTypeDropinDir
		} else {
			res.Kind = components.ResourceTypeDirectory
		}
		return res, nil
	}
	if rt.IsQuadlet() && rt != components.ResourceTypeDropin {
		data, err := root.ReadFile(path)
		if err != nil {
			return res, err
		}
		if res.HostObject, err = res.GetHostObject(string(data)); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (r *HostComponentRepository) ReadResource(res components.Resource) (string, error) {
	if err := res.Validate(); err != nil {
		return "", fmt.Errorf("can't read invalid resource %v: %w", res.Path, err)
	}
	if res.Kind == components.ResourceTypeDirectory || res.Kind == components.ResourceTypeDropinDir {
		return "", nil
	}
	if res.Kind == components.ResourceTypePodmanSecret {
		return "", errors.New("secrets don't live in repositories")
	}

	prefix := r.dataPrefix
	if inQuadletDir(res) {
		prefix = r.quadletPrefix
	}
	root, err := openRes(prefix, res)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()
	curFile, err := root.ReadFile(res.Path)
	if err != nil {
		return "", err
	}

	return string(curFile), nil
}

func (r *HostComponentRepository) InstallResource(res components.Resource, data []byte) error {
	if err := res.Validate(); err != nil {
		return fmt.Errorf("can't install invalid resource %v: %w", res.Path, err)
	}

	prefix := r.dataPrefix
	mode := fs.FileMode(0o755) // TODO pull permissions from source dir

	if inQuadletDir(res) {
		prefix = r.quadletPrefix
		mode = 0o644
	}
	root, err := openRes(prefix, res)
	if err != nil {
		return err
	}

	defer func() { _ = root.Close() }()
	if res.Kind == components.ResourceTypeDirectory || res.Kind == components.ResourceTypeDropinDir {
		err := root.Mkdir(res.Path, mode)
		if err != nil {
			return err
		}
		return nil
	}
	return atomicWrite(root, res.Path, mode, data)
}

func (r *HostComponentRepository) RemoveResource(res components.Resource) error {
	if err := res.Validate(); err != nil {
		return fmt.Errorf("can't remove invalid resource %v: %w", res.Path, err)
	}
	prefix := r.dataPrefix
	if inQuadletDir(res) {
		prefix = r.quadletPrefix
	}
	root, err := openRes(prefix, res)
	if err != nil {
		return err
	}

	return root.Remove(res.Path)
}

func (r *HostComponentRepository) ComponentExists(name string) (bool, error) {
	droot, err := r.dRoot()
	if err != nil {
		return false, err
	}
	defer func() { _ = droot.Close() }()
	_, err = droot.Stat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *HostComponentRepository) PurgeComponent(c *components.Component) error {
	if c == nil {
		return errors.New("no component specified")
	}
	return r.PurgeComponentByName(c.InstanceName())
}

func (r *HostComponentRepository) PurgeComponentByName(name string) error {
	if name == "" {
		return errors.New("no component specified")
	}

	if err := components.ValidateComponentName(name); err != nil {
		return err
	}

	droot, err := r.dRoot()
	if err != nil {
		return err
	}
	defer func() { _ = droot.Close() }()

	qroot, err := r.qRoot()
	if err != nil {
		return err
	}
	defer func() { _ = qroot.Close() }()
	if err := droot.RemoveAll(name); err != nil {
		return err
	}

	return qroot.RemoveAll(name)
}

func (r *HostComponentRepository) Clean() error {
	if err := r.cleanQuadlets(); err != nil {
		return err
	}
	return r.cleanData()
}

func (r *HostComponentRepository) cleanQuadlets() error {
	root, err := r.qRoot()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	defer func() { _ = root.Close() }()

	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Only touch directories Materia created.
		_, err := root.Stat(filepath.Join(e.Name(), ".materia_managed"))
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, err)
			}
			continue
		}
		if err := root.RemoveAll(e.Name()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (r *HostComponentRepository) cleanData() error {
	root, err := r.dRoot()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	defer func() { _ = root.Close() }()

	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if err := root.RemoveAll(e.Name()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func inQuadletDir(res components.Resource) bool {
	return res.IsQuadlet() || res.Kind == components.ResourceTypeDropinDir
}
