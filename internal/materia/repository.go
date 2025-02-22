package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/secrets"
	"github.com/charmbracelet/log"
)

type Repository interface {
	Setup(context.Context) error
	GetSourceResource(context.Context, string, string) (*Resource, error)
	GetAllSourceResources(context.Context, string) ([]*Resource, error)
	GetInstalledResource(context.Context, string, string) (*Resource, error)
	GetAllInstalledResources(context.Context, string) ([]*Resource, error)
	GetSourceComponent(context.Context, string) (*Component, error)
	GetAllSourceComponents(context.Context) ([]*Component, error)
	GetInstalledComponent(context.Context, string) (*Component, error)
	GetAllInstalledComponents(context.Context) ([]*Component, error)
	InstallResource(ctx context.Context, comp *Component, res Resource, funcMap func(map[string]interface{}) template.FuncMap, vars map[string]interface{}) error
	RemoveResource(comp *Component, res Resource, _ secrets.SecretsManager) error
	InstallComponent(comp *Component, _ secrets.SecretsManager) error
	RemoveComponent(comp *Component, _ secrets.SecretsManager) error
	SourcePath() string
	DataPath(string) string
	GetManifest(context.Context) (*MateriaManifest, error)
	GetComponentManifest(context.Context, string) (*ComponentManifest, error)
	Clean(context.Context) error
}

type FileRepository struct {
	// defaults: /var/lib/materia, /etc/containers/systemd, /var/lib/materia/components, /var/lib/materia/source, /usr/local/bin/, /etc/systemd/system
	prefix, quadletDestination, data, source, scriptsLocation, servicesLocation string
	debug                                                                       bool
}

func (f *FileRepository) Setup(_ context.Context) error {
	if _, err := os.Stat(filepath.Join(f.source, "MANIFEST.toml")); err != nil {
		return fmt.Errorf("no manifest found: %w", err)
	}
	if res, err := os.Stat(filepath.Join(f.source, "components")); err != nil {
		return fmt.Errorf("no components found: %w", err)
	} else {
		if !res.IsDir() {
			return errors.New("source components is not a directory")
		}
	}
	if _, err := os.Stat(f.prefix); os.IsNotExist(err) {
		return fmt.Errorf("prefix %v does not exist, setup manually", f.prefix)
	}
	if _, err := os.Stat(f.quadletDestination); os.IsNotExist(err) {
		return fmt.Errorf("destination %v does not exist, setup manually", f.quadletDestination)
	}
	if _, err := os.Stat(f.scriptsLocation); os.IsNotExist(err) {
		return fmt.Errorf("scripts location %v does not exist, setup manually", f.scriptsLocation)
	}
	if _, err := os.Stat(f.servicesLocation); os.IsNotExist(err) {
		return fmt.Errorf("services location %v does not exist, setup manually", f.servicesLocation)
	}
	err := os.Mkdir(filepath.Join(f.prefix), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("error creating prefix: %w", err)
	}
	err = os.Mkdir(filepath.Join(f.prefix, "source"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("error creating source repo: %w", err)
	}
	err = os.Mkdir(filepath.Join(f.prefix, "components"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("error creating components in prefix: %w", err)
	}
	return nil
}

func (f *FileRepository) GetSourceResource(_ context.Context, parent, name string) (*Resource, error) {
	panic("not implemented") // TODO: Implement
}

func (f *FileRepository) GetAllSourceResources(_ context.Context, componentName string) ([]*Resource, error) {
	var resources []*Resource
	compPath := filepath.Join(f.source, "components", componentName)
	entries, err := os.ReadDir(compPath)
	if err != nil {
		return nil, err
	}
	for _, v := range entries {
		resources = append(resources, &Resource{
			Path:     filepath.Join(compPath, v.Name()),
			Name:     strings.TrimSuffix(v.Name(), ".gotmpl"),
			Kind:     findResourceType(v.Name()),
			Template: isTemplate(v.Name()),
		})
	}
	return resources, nil
}

func (f *FileRepository) GetInstalledResource(_ context.Context, _, _ string) (*Resource, error) {
	panic("not implemented") // TODO: Implement
}

func (f *FileRepository) GetAllInstalledResources(_ context.Context, componentName string) ([]*Resource, error) {
	var resources []*Resource
	compPath := filepath.Join(f.quadletDestination, componentName)
	dataPath := filepath.Join(f.data, componentName)
	entries, err := os.ReadDir(compPath)
	if err != nil {
		return nil, err
	}
	for _, v := range entries {
		resources = append(resources, &Resource{
			Path:     filepath.Join(compPath, v.Name()),
			Name:     v.Name(),
			Kind:     findResourceType(v.Name()),
			Template: isTemplate(v.Name()),
		})
	}
	entries, err = os.ReadDir(dataPath)
	if err != nil {
		return nil, err
	}
	for _, v := range entries {
		resources = append(resources, &Resource{
			Path:     filepath.Join(dataPath, v.Name()),
			Name:     v.Name(),
			Kind:     findResourceType(v.Name()),
			Template: isTemplate(v.Name()),
		})
	}
	return resources, nil
}

func (f *FileRepository) GetSourceComponent(_ context.Context, componentName string) (*Component, error) {
	compPath := filepath.Join(f.source, "components", componentName)
	return NewComponentFromSource(compPath)
}

func (f *FileRepository) GetAllSourceComponents(_ context.Context) ([]*Component, error) {
	var components []*Component
	compPath := filepath.Join(f.source, "components")
	var compPaths []string
	entries, err := os.ReadDir(compPath)
	if err != nil {
		return nil, err
	}
	for _, v := range entries {
		if v.IsDir() {
			compPaths = append(compPaths, v.Name())
		}
	}
	for _, v := range compPaths {
		c, err := NewComponentFromSource(filepath.Join(compPath, v))
		if err != nil {
			return nil, err
		}
		components = append(components, c)
	}
	return components, nil
}

func (f *FileRepository) GetInstalledComponent(_ context.Context, name string) (*Component, error) {
	oldComp := &Component{
		Name:      name,
		Resources: []Resource{},
		State:     StateStale,
		Services:  []Resource{},
		Defaults:  make(map[string]interface{}),
	}
	// load resources

	var man *ComponentManifest
	entries, err := os.ReadDir(filepath.Join(f.prefix, "components", name))
	if err != nil {
		return nil, err
	}
	scripts := 0
	for _, r := range entries {
		newRes := Resource{
			Path:     filepath.Join(filepath.Join(f.prefix, "components", name, r.Name())),
			Name:     strings.TrimSuffix(r.Name(), ".gotmpl"),
			Kind:     findResourceType(r.Name()),
			Template: isTemplate(r.Name()),
		}

		oldComp.Resources = append(oldComp.Resources, newRes)
		if r.Name() == "MANIFEST.toml" {
			log.Debugf("loading installed component manifest %v", oldComp.Name)
			man, err = LoadComponentManifest(newRes.Path)
			if err != nil {
				return nil, fmt.Errorf("error loading component manifest: %w", err)
			}
			maps.Copy(oldComp.Defaults, man.Defaults)
		}
		if r.Name() == "setup.sh" || r.Name() == "cleanup.sh" {
			scripts++
			oldComp.Scripted = true
		}

	}

	if scripts != 0 && scripts != 2 {
		return nil, errors.New("scripted component is missing install or cleanup")
	}
	// load quadlets
	entries, err = os.ReadDir(filepath.Join(f.quadletDestination, name))
	if err != nil {
		return nil, err
	}
	for _, r := range entries {
		if r.Name() == ".materia_managed" {
			continue
		}
		newRes := Resource{
			Path:     filepath.Join(f.quadletDestination, name, r.Name()),
			Name:     strings.TrimSuffix(r.Name(), ".gotmpl"),
			Kind:     findResourceType(r.Name()),
			Template: isTemplate(r.Name()),
		}
		oldComp.Resources = append(oldComp.Resources, newRes)
	}
	if man != nil {
		for _, s := range man.Services {
			if s == "" || (!strings.HasSuffix(s, ".service") && !strings.HasSuffix(s, ".target") && !strings.HasSuffix(s, ".timer")) {
				return nil, fmt.Errorf("error loading component services: invalid service format %v", s)
			}
			oldComp.Services = append(oldComp.Services, Resource{
				Name: s,
				Kind: ResourceTypeService,
			})
		}
	}

	return oldComp, nil
}

func (f *FileRepository) GetAllInstalledComponents(ctx context.Context) ([]*Component, error) {
	var components []*Component
	intalledComponents := filepath.Join(f.prefix, "components")
	entries, err := os.ReadDir(intalledComponents)
	if err != nil {
		return nil, err
	}
	for _, v := range entries {
		oldComp, err := f.GetInstalledComponent(ctx, v.Name())
		if err != nil {
			return nil, err
		}
		components = append(components, oldComp)
	}
	return components, nil
}

func (f *FileRepository) installFile(path string, data *bytes.Buffer) error {
	err := os.WriteFile(path, data.Bytes(), 0o755)
	if err != nil {
		return err
	}
	return nil
}

func (f *FileRepository) linkFile(path, destination string) error {
	return os.Symlink(path, destination)
}

func (f *FileRepository) installPath(comp *Component, r Resource) string {
	switch r.Kind {
	case ResourceTypeManifest, ResourceTypeFile, ResourceTypeScript, ResourceTypeComponentScript, ResourceTypeService:
		return filepath.Join(f.prefix, "components", comp.Name)
	case ResourceTypeContainer, ResourceTypeKube, ResourceTypeNetwork, ResourceTypePod, ResourceTypeVolume:
		return filepath.Join(f.quadletDestination, comp.Name)
	case ResourceTypeVolumeFile:
		// TODO I guess this needs to be handled?
		return "/tmp/"
	default:
		panic("calculating install path for unknown resource")
	}
}

func (f *FileRepository) InstallComponent(comp *Component, _ secrets.SecretsManager) error {
	if err := comp.Validate(); err != nil {
		return err
	}

	if comp.State != StateFresh && comp.State != StateOK {
		return errors.New("tried to install a stale component")
	}

	err := os.Mkdir(filepath.Join(f.prefix, "components", comp.Name), 0o755)
	if err != nil {
		return fmt.Errorf("error installing component %v: %w", filepath.Join(f.prefix, "components", comp.Name), err)
	}
	qpath := filepath.Join(f.quadletDestination, comp.Name)
	spath := filepath.Join(f.servicesLocation, comp.Name)
	err = os.Mkdir(qpath, 0o755)
	if err != nil {
		return fmt.Errorf("error installing component: %w", err)
	}

	err = os.Mkdir(filepath.Join(f.servicesLocation, comp.Name), 0o755)
	if err != nil {
		return fmt.Errorf("error installing component services: %w", err)
	}
	qFile, err := os.OpenFile(fmt.Sprintf("%v/.materia_managed", qpath), os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("error installing component: %w", err)
	}
	defer qFile.Close()
	sFile, err := os.OpenFile(fmt.Sprintf("%v/.materia_managed", spath), os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("error installing component: %w", err)
	}
	defer sFile.Close()

	return nil
}

func (f *FileRepository) RemoveComponent(comp *Component, _ secrets.SecretsManager) error {
	if err := comp.Validate(); err != nil {
		return err
	}
	for _, v := range comp.Resources {
		err := os.Remove(v.Path)
		if err != nil {
			return err
		}
		log.Info("removed", "resource", v.Name)
	}

	err := os.Remove(filepath.Join(f.data, comp.Name))
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(f.quadletDestination, comp.Name, ".materia_managed"))
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(f.quadletDestination, comp.Name))
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(f.servicesLocation, comp.Name, ".materia_managed"))
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(f.servicesLocation, comp.Name))
	return err
}

func (f *FileRepository) InstallResource(ctx context.Context, comp *Component, res Resource, funcMap func(map[string]interface{}) template.FuncMap, vars map[string]interface{}) error {
	if err := comp.Validate(); err != nil {
		return err
	}
	if err := res.Validate(); err != nil {
		return err
	}
	path := f.installPath(comp, res)
	var result *bytes.Buffer
	data, err := os.ReadFile(res.Path)
	if err != nil {
		return err
	}
	if res.Template {
		log.Debug("applying template", "file", res.Name)
		result, err = res.execute(funcMap, vars)
		if err != nil {
			return err
		}
	} else {
		result = bytes.NewBuffer(data)
	}
	fileLocation := fmt.Sprintf("%v/%v", path, res.Name)
	err = f.installFile(fileLocation, result)
	if err != nil {
		return err
	}
	if res.Kind == ResourceTypeScript || res.Kind == ResourceTypeComponentScript {
		err = os.Chmod(fileLocation, 0755)
		if err != nil {
			return err
		}
	} else {
		err = os.Chmod(fileLocation, 0644)
		if err != nil {
			return err
		}
	}
	if res.Kind == ResourceTypeScript {
		err = f.installFile(fmt.Sprintf("%v/%v", f.scriptsLocation, res.Name), result)
		if err != nil {
			return err
		}
	}
	if res.Kind == ResourceTypeService {
		err = f.installFile(fmt.Sprintf("%v/%v", f.servicesLocation, res.Name), result)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *FileRepository) RemoveResource(comp *Component, res Resource, _ secrets.SecretsManager) error {
	if err := comp.Validate(); err != nil {
		return err
	}
	if err := res.Validate(); err != nil {
		return err
	}
	if strings.Contains(res.Path, f.source) {
		return fmt.Errorf("tried to remove resource %v for component %v from source", res.Name, comp.Name)
	}

	if res.Kind == ResourceTypeScript {
		err := os.Remove(path.Join(f.scriptsLocation, res.Name))
		if err != nil {
			return err
		}
	}
	if res.Kind == ResourceTypeService {
		err := os.Remove(path.Join(f.servicesLocation, res.Name))
		if err != nil {
			return err
		}
	}

	err := os.Remove(res.Path)
	if err != nil {
		return err
	}
	return nil
}

func (f *FileRepository) deleteWrapper(path string) error {
	if f.debug {
		log.Debug("would delete path", "path", path)
		return nil
	}
	return os.RemoveAll(path)
}

func (f *FileRepository) Clean(ctx context.Context) error {
	err := f.deleteWrapper(f.source)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(f.quadletDestination)
	if err != nil {
		return err
	}
	for _, v := range entries {
		if v.IsDir() {
			_, err := os.Stat(fmt.Sprintf("%v/%v/.materia_managed", f.quadletDestination, v.Name()))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			err = f.deleteWrapper(fmt.Sprintf("%v/%v", f.quadletDestination, v.Name()))
			if err != nil {
				return err
			}
		}
	}
	err = f.deleteWrapper(filepath.Join(f.data))
	if err != nil {
		return err
	}
	// TODO remove scripts too?
	// clean prefix too?
	return nil
}

func (f *FileRepository) GetManifest(ctx context.Context) (*MateriaManifest, error) {
	return LoadMateriaManifest(filepath.Join(f.source, "MANIFEST.toml"))
}

func (f *FileRepository) GetComponentManifest(ctx context.Context, component string) (*ComponentManifest, error) {
	return LoadComponentManifest(filepath.Join(f.source, "components", component, "MANIFEST.toml"))
}

func (f *FileRepository) SourcePath() string {
	return f.source
}

func (f *FileRepository) DataPath(c string) string {
	return filepath.Join(f.prefix, "components", c)
}

func NewFileRepository(p, q, d, s, source string, debug bool) *FileRepository {
	if strings.HasSuffix(p, "materia") {
		panic("BAD PREFIX")
	}
	return &FileRepository{
		prefix:             filepath.Join(p, "materia"),
		quadletDestination: q,
		data:               d,
		servicesLocation:   s,
		source:             source,
		scriptsLocation:    "/usr/local/bin",
		debug:              debug,
	}
}
