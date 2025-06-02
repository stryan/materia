package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"git.saintnet.tech/stryan/materia/internal/components"
	"git.saintnet.tech/stryan/materia/internal/containers"
	"git.saintnet.tech/stryan/materia/internal/manifests"
	"git.saintnet.tech/stryan/materia/internal/repository"
	"git.saintnet.tech/stryan/materia/internal/secrets"
	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	"git.saintnet.tech/stryan/materia/internal/secrets/mem"
	"git.saintnet.tech/stryan/materia/internal/services"
	"git.saintnet.tech/stryan/materia/internal/source"
	"git.saintnet.tech/stryan/materia/internal/source/file"
	"git.saintnet.tech/stryan/materia/internal/source/git"
	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
)

type MacroMap func(map[string]any) template.FuncMap

type Materia struct {
	Facts         *Facts
	Manifest      *manifests.MateriaManifest
	Services      services.Services
	PodmanConn    context.Context
	Containers    containers.ContainerManager
	sm            secrets.SecretsManager
	source        source.Source
	CompRepo      repository.ComponentRepository
	ScriptRepo    repository.Repository
	ServiceRepo   repository.Repository
	SourceRepo    repository.ComponentRepository
	rootComponent *components.Component
	macros        MacroMap
	snippets      map[string]*Snippet
	OutputDir     string
	onlyResources bool
	debug         bool
	diffs         bool
	cleanup       bool
}

func NewMateria(ctx context.Context, c *Config, sm services.Services, cm containers.ContainerManager) (*Materia, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if _, err := os.Stat(c.QuadletDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("destination %v does not exist, setup manually", c.QuadletDir)
	}
	if _, err := os.Stat(c.ScriptDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("scripts location %v does not exist, setup manually", c.ScriptDir)
	}
	if _, err := os.Stat(c.ServiceDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("services location %v does not exist, setup manually", c.ServiceDir)
	}

	err := os.Mkdir(filepath.Join(c.MateriaDir, "materia"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating prefix: %w", err)
	}
	err = os.Mkdir(c.OutputDir, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating output dir: %w", err)
	}
	err = os.Mkdir(c.SourceDir, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating source repo: %w", err)
	}
	err = os.Mkdir(filepath.Join(c.MateriaDir, "materia", "components"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating components in prefix: %w", err)
	}

	var source source.Source
	parsedPath := strings.Split(c.SourceURL, "://")
	switch parsedPath[0] {
	case "git":
		source, err = git.NewGitSource(c.SourceDir, parsedPath[1], c.GitConfig)
		if err != nil {
			return nil, fmt.Errorf("invalid git source: %w", err)
		}
	case "file":
		source = file.NewFileSource(c.SourceDir, parsedPath[1])
	default:
		return nil, fmt.Errorf("invalid source: %v", parsedPath[0])
	}

	// Ensure local cache
	if c.NoSync {
		log.Debug("skipping cache update on request")
	} else {
		log.Debug("updating configured source cache")
		err = source.Sync(ctx)
		if err != nil {
			return nil, fmt.Errorf("error syncing source: %w", err)
		}
	}
	log.Debug("loading manifest")
	man, err := manifests.LoadMateriaManifest(filepath.Join(c.SourceDir, "MANIFEST.toml"))
	if err != nil {
		return nil, fmt.Errorf("error loading manifest: %w", err)
	}
	if err := man.Validate(); err != nil {
		return nil, err
	}

	log.Debug("loading facts")
	compRepo := &repository.HostComponentRepository{DataPrefix: filepath.Join(c.MateriaDir, "materia", "components"), QuadletPrefix: c.QuadletDir}
	facts, err := NewFacts(ctx, c, man, compRepo, cm)
	if err != nil {
		return nil, fmt.Errorf("error generating facts: %w", err)
	}
	snips := make(map[string]*Snippet)
	defaultSnippets := loadDefaultSnippets()
	for _, v := range defaultSnippets {
		snips[v.Name] = v
	}
	m := &Materia{
		Services:      sm,
		Containers:    cm,
		Facts:         facts,
		Manifest:      man,
		source:        source,
		debug:         c.Debug,
		diffs:         c.Diffs,
		cleanup:       c.Cleanup,
		onlyResources: c.OnlyResources,
		CompRepo:      compRepo,
		ScriptRepo:    &repository.FileRepository{Prefix: c.ScriptDir},
		ServiceRepo:   &repository.FileRepository{Prefix: c.ServiceDir},
		SourceRepo:    &repository.SourceComponentRepository{Prefix: filepath.Join(c.SourceDir, "components")},
		OutputDir:     c.OutputDir,
		snippets:      snips,
		rootComponent: &components.Component{Name: "root"},
	}
	m.macros = func(vars map[string]any) template.FuncMap {
		return template.FuncMap{
			"m_deps": func(arg string) (string, error) {
				switch arg {
				case "after":
					if res, ok := vars["After"]; ok {
						return res.(string), nil
					} else {
						return "local-fs.target network.target", nil
					}
				case "wants":
					if res, ok := vars["Wants"]; ok {
						return res.(string), nil
					} else {
						return "local-fs.target network.target", nil
					}
				case "requires":
					if res, ok := vars["Requires"]; ok {
						return res.(string), nil
					} else {
						return "local-fs.target network.target", nil
					}
				default:
					return "", errors.New("err bad default")
				}
			},
			"m_dataDir": func(arg string) (string, error) {
				return filepath.Join(compRepo.DataPrefix, arg), nil
			},
			"m_facts": func(arg string) (any, error) {
				return m.Facts.Lookup(arg)
			},
			"m_default": func(arg string, def string) string {
				val, ok := vars[arg]
				if ok {
					return val.(string)
				}
				return def
			},
			"exists": func(arg string) bool {
				_, ok := vars[arg]
				return ok
			},
			"snippet": func(name string, args ...string) (string, error) {
				s, ok := m.snippets[name]
				if !ok {
					return "", errors.New("snippet not found")
				}
				snipVars := make(map[string]string, len(s.Parameters))
				for k, v := range s.Parameters {
					snipVars[v] = args[k]
				}

				result := bytes.NewBuffer([]byte{})
				err := s.Body.Execute(result, snipVars)
				return result.String(), err
			},
		}
	}

	switch m.Manifest.Secrets {
	case "age":
		conf, ok := m.Manifest.SecretsConfig.(age.Config)
		if !ok {
			return nil, errors.New("tried to create an age secrets manager but config was not for age")
		}
		conf.RepoPath = c.SourceDir
		if c.AgeConfig != nil {
			conf.Merge(c.AgeConfig)
		}
		m.sm, err = age.NewAgeStore(conf)
		if err != nil {
			return nil, fmt.Errorf("error creating age store: %w", err)
		}

	case "mem":
		m.sm = mem.NewMemoryManager()
	default:
		m.sm = mem.NewMemoryManager()
	}
	for _, v := range m.Manifest.Snippets {
		s, err := configToSnippet(v)
		if err != nil {
			return nil, err
		}
		m.snippets[s.Name] = s
	}
	return m, nil
}

func (m *Materia) Close() {
	m.Services.Close()
	m.Containers.Close()
}

func (m *Materia) updateComponents() (map[string]*components.Component, error) {
	updatedComponents := make(map[string]*components.Component)

	// figure out ones to add
	var found []string
	sourcePaths, err := m.SourceRepo.ListComponentNames()
	if err != nil {
		return nil, fmt.Errorf("error getting source components: %w", err)
	}
	var sourceComps []*components.Component
	for _, v := range sourcePaths {
		comp, err := m.SourceRepo.GetComponent(v)
		if err != nil {
			return nil, fmt.Errorf("error creating component from source: %w", err)
		}
		sourceComps = append(sourceComps, comp)
	}
	currentComps := make(map[string]*components.Component)
	maps.Copy(currentComps, m.Facts.InstalledComponents)
	for _, v := range sourceComps {
		if !slices.Contains(m.Facts.AssignedComponents, v.Name) {
			// not assigned to host, skip
			continue
		}
		found = append(found, v.Name)
		_, ok := currentComps[v.Name]
		if !ok {
			v.State = components.StateFresh
		} else {
			v.State = components.StateMayNeedUpdate
			delete(currentComps, v.Name)
		}
		updatedComponents[v.Name] = v
	}
	for _, v := range currentComps {
		v.State = components.StateNeedRemoval
		updatedComponents[v.Name] = v
	}
	if len(found) != len(m.Facts.AssignedComponents) {
		log.Debugf("New Components: %v Assigned Components: %v", found, m.Facts.AssignedComponents)
		return nil, fmt.Errorf("not all assigned components were found for this host")
	}

	return updatedComponents, nil
}

func (m *Materia) modifyService(ctx context.Context, command Action) error {
	if err := command.Validate(); err != nil {
		return err
	}
	var res components.Resource
	if command.Todo != ActionReloadUnits {
		res = command.Payload
		if err := res.Validate(); err != nil {
			return fmt.Errorf("invalid resource when modifying service: %w", err)
		}

		if res.Kind != components.ResourceTypeService {
			return errors.New("attempted to modify a non service resource")
		}
	}
	var cmd services.ServiceAction
	switch command.Todo {
	case ActionStartService:
		cmd = services.ServiceStart
		log.Debug("starting service", "unit", res.Name)
	case ActionStopService:
		log.Debug("stopping service", "unit", res.Name)
		cmd = services.ServiceStop
	case ActionRestartService:
		log.Debug("restarting service", "unit", res.Name)
		cmd = services.ServiceRestart
	case ActionReloadUnits:
		log.Debug("reloading units")
		cmd = services.ServiceReloadUnits
	case ActionEnableService:
		log.Debug("enabling service", "unit", res.Name)
		cmd = services.ServiceEnable
	case ActionDisableService:
		log.Debug("disabling service", "unit", res.Name)
		cmd = services.ServiceDisable
	case ActionReloadService:
		log.Debug("reloading service", "unit", res.Name)
		cmd = services.ServiceReloadService

	default:
		return errors.New("invalid service command")
	}
	return m.Services.Apply(ctx, res.Name, cmd)
}

func (m *Materia) Plan(ctx context.Context) (*Plan, error) {
	plan := NewPlan(m.Facts)
	var err error

	if len(m.Facts.InstalledComponents) == 0 && len(m.Facts.AssignedComponents) == 0 {
		return plan, nil
	}

	var newComponents map[string]*components.Component
	log.Debug("calculating component differences")
	if newComponents, err = m.updateComponents(); err != nil {
		return plan, fmt.Errorf("error determining components: %w", err)
	}
	// Determine diff actions
	log.Debug("calculating resource differences")
	finalComponents, err := m.calculateDiffs(ctx, newComponents, plan)
	if err != nil {
		return plan, fmt.Errorf("error calculating diffs: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("generated invalid plan: %w", err)
	}
	var installing, removing, updating, ok []string
	keys := sortedKeys(finalComponents)
	for _, k := range keys {
		v := finalComponents[k]
		switch v.State {
		case components.StateFresh:
			installing = append(installing, v.Name)
			log.Debug("fresh:", "component", v.Name)
		case components.StateNeedUpdate:
			updating = append(updating, v.Name)
			log.Debug("updating:", "component", v.Name)
		case components.StateMayNeedUpdate:
			log.Warn("component still listed as may need update", "component", v.Name)
		case components.StateNeedRemoval:
			removing = append(removing, v.Name)
			log.Debug("remove:", "component", v.Name)
		case components.StateOK:
			ok = append(ok, v.Name)
			log.Debug("ok:", "component", v.Name)
		case components.StateRemoved:
			log.Debug("removed:", "component", v.Name)
		case components.StateStale:
			log.Debug("stale:", "component", v.Name)
		case components.StateUnknown:
			log.Debug("unknown:", "component", v.Name)
		default:
			panic(fmt.Sprintf("unexpected main.ComponentLifecycle: %#v", v.State))
		}
	}

	log.Debug("installing components", "installing", installing)
	log.Debug("removing components", "removing", removing)
	log.Debug("updating components", "updating", updating)
	log.Debug("unchanged components", "unchanged", ok)

	return plan, nil
}

func (m *Materia) Execute(ctx context.Context, plan *Plan) (int, error) {
	if plan.Empty() {
		return -1, nil
	}
	defer func() {
		if !m.cleanup {
			return
		}
		problems, err := m.ValidateComponents(ctx)
		if err != nil {
			log.Warnf("error cleaning up execution: %v", err)
		}
		for _, v := range problems {
			log.Infof("component %v failed to install, purging", v)
			err := m.CompRepo.PurgeComponentByName(v)
			if err != nil {
				log.Warnf("error purging component: %v", err)
			}
		}
	}()
	serviceActions := []Action{}
	steps := 0
	// Template and install resources
	for _, v := range plan.Steps() {
		vars := make(map[string]any)
		if err := v.Validate(); err != nil {
			return steps, err
		}
		vaultVars := m.sm.Lookup(ctx, secrets.SecretFilter{
			Hostname:  m.Facts.Hostname,
			Roles:     m.Facts.Roles,
			Component: v.Parent.Name,
		})
		maps.Copy(vars, v.Parent.Defaults)
		maps.Copy(vars, vaultVars)
		err := m.executeAction(ctx, v, vars)
		if err != nil {
			return steps, err
		}

		if v.Todo == ActionStartService || v.Todo == ActionStopService || v.Todo == ActionRestartService || v.Todo == ActionEnableService || v.Todo == ActionDisableService {
			serviceActions = append(serviceActions, v)
		}

		steps++
	}

	// verify services
	activating := []string{}
	deactivating := []string{}
	for _, v := range serviceActions {
		serv, err := m.Services.Get(ctx, v.Payload.Name)
		if err != nil {
			return steps, err
		}
		switch v.Todo {
		case ActionRestartService, ActionStartService:
			if serv.State == "activating" {
				activating = append(activating, v.Payload.Name)
			} else if serv.State != "active" {
				log.Warn("service failed to start/restart", "service", serv.Name, "state", serv.State)
			}
		case ActionStopService:
			if serv.State == "deactivating" {
				deactivating = append(deactivating, v.Payload.Name)
			} else if serv.State != "inactive" {
				log.Warn("service failed to stop", "service", serv.Name, "state", serv.State)
			}
		case ActionEnableService, ActionDisableService:
		default:
			return steps, errors.New("unknown service action state")
		}
	}
	var servWG sync.WaitGroup
	for _, v := range activating {
		servWG.Add(1)
		go func() {
			defer servWG.Done()
			err := m.Services.WaitUntilState(ctx, v, "active")
			if err != nil {
				log.Warn(err)
			}
		}()
	}
	for _, v := range deactivating {
		servWG.Add(1)
		go func() {
			defer servWG.Done()
			err := m.Services.WaitUntilState(ctx, v, "inactive")
			if err != nil {
				log.Warn(err)
			}
		}()
	}
	servWG.Wait()
	return steps, nil
}

func (m *Materia) executeAction(ctx context.Context, v Action, vars map[string]any) error {
	switch v.Todo {
	case ActionInstallComponent:
		if err := m.CompRepo.InstallComponent(v.Parent); err != nil {
			return err
		}
	case ActionInstallFile, ActionUpdateFile, ActionInstallQuadlet, ActionUpdateQuadlet:
		resourceTemplate, err := m.SourceRepo.ReadResource(v.Payload)
		if err != nil {
			return err
		}
		resourceData, err := m.executeResource(resourceTemplate, vars)
		if err != nil {
			return err
		}
		if err := m.CompRepo.InstallResource(v.Payload, resourceData); err != nil {
			return err
		}
	case ActionInstallScript, ActionUpdateScript:
		resourceTemplate, err := m.SourceRepo.ReadResource(v.Payload)
		if err != nil {
			return err
		}
		resourceData, err := m.executeResource(resourceTemplate, vars)
		if err != nil {
			return err
		}
		if err := m.CompRepo.InstallResource(v.Payload, resourceData); err != nil {
			return err
		}
		if err := m.ScriptRepo.Install(ctx, v.Payload.Name, resourceData); err != nil {
			return err
		}
	case ActionInstallService, ActionUpdateService:
		resourceTemplate, err := m.SourceRepo.ReadResource(v.Payload)
		if err != nil {
			return err
		}
		resourceData, err := m.executeResource(resourceTemplate, vars)
		if err != nil {
			return err
		}
		if err := m.CompRepo.InstallResource(v.Payload, resourceData); err != nil {
			return err
		}
		if err := m.ServiceRepo.Install(ctx, v.Payload.Name, resourceData); err != nil {
			return err
		}
	case ActionInstallComponentScript, ActionUpdateComponentScript:
		resourceTemplate, err := m.SourceRepo.ReadResource(v.Payload)
		if err != nil {
			return err
		}
		resourceData, err := m.executeResource(resourceTemplate, vars)
		if err != nil {
			return err
		}
		if err := m.CompRepo.InstallResource(v.Payload, resourceData); err != nil {
			return err
		}
	case ActionRemoveFile:
		if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
			return err
		}
	case ActionRemoveQuadlet:
		if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
			return err
		}
	case ActionRemoveScript:
		if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
			return err
		}
		if err := m.ScriptRepo.Remove(ctx, v.Payload.Name); err != nil {
			return err
		}
	case ActionRemoveService:
		if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
			return err
		}
		if err := m.ServiceRepo.Remove(ctx, v.Payload.Name); err != nil {
			return err
		}
	case ActionRemoveComponentScript:
		if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
			return err
		}
	case ActionRemoveComponent:
		if err := m.CompRepo.RemoveComponent(v.Parent); err != nil {
			return err
		}
	case ActionCleanupComponent:
		if err := m.CompRepo.RunCleanup(v.Parent); err != nil {
			return err
		}
	case ActionEnsureVolume:
		service := strings.TrimSuffix(v.Payload.Name, ".volume")
		err := m.modifyService(ctx, Action{
			Todo:   ActionStartService,
			Parent: v.Parent,
			Payload: components.Resource{
				Parent: v.Parent.Name,
				Name:   fmt.Sprintf("%v-volume.service", service),
				Kind:   components.ResourceTypeService,
			},
		})
		if err != nil {
			return err
		}
	case ActionInstallVolumeFile:
		resourceTemplate, err := m.SourceRepo.ReadResource(v.Payload)
		if err != nil {
			return err
		}
		resourceData, err := m.executeResource(resourceTemplate, vars)
		if err != nil {
			return err
		}
		if err := m.CompRepo.InstallResource(v.Payload, resourceData); err != nil {
			return err
		}
		if err := m.InstallVolumeFile(ctx, v.Parent, v.Payload); err != nil {
			return err
		}
	case ActionUpdateVolumeFile:
		resourceTemplate, err := m.SourceRepo.ReadResource(v.Payload)
		if err != nil {
			return err
		}
		resourceData, err := m.executeResource(resourceTemplate, vars)
		if err != nil {
			return err
		}
		if err := m.CompRepo.InstallResource(v.Payload, resourceData); err != nil {
			return err
		}
		if err := m.InstallVolumeFile(ctx, v.Parent, v.Payload); err != nil {
			return err
		}
	case ActionRemoveVolumeFile:
		if err := m.CompRepo.RemoveResource(v.Payload); err != nil {
			return err
		}
		if err := m.RemoveVolumeFile(ctx, v.Parent, v.Payload); err != nil {
			return err
		}
	case ActionSetupComponent:
		if err := m.CompRepo.RunSetup(v.Parent); err != nil {
			return err
		}
	case ActionStartService, ActionStopService, ActionRestartService, ActionEnableService, ActionDisableService:
		err := m.modifyService(ctx, v)
		if err != nil {
			return err
		}
	case ActionReloadUnits:
		err := m.modifyService(ctx, v)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid action to execute: %v", v)
	}
	return nil
}

func (m *Materia) InstallVolumeFile(ctx context.Context, parent *components.Component, res components.Resource) error {
	var vrConf *manifests.VolumeResourceConfig
	for _, vr := range parent.VolumeResources {
		if vr.Resource == res.Name {
			vrConf = &vr
			break
		}
	}
	if vrConf == nil {
		return fmt.Errorf("tried to install volume file for nonexistent volume resource: %v", res.Name)
	}
	vrConf.Volume = fmt.Sprintf("systemd-%v", vrConf.Volume)
	volumes, err := m.Containers.ListVolumes(ctx)
	if err != nil {
		return err
	}
	var volume *containers.Volume
	if !slices.ContainsFunc(volumes, func(v *containers.Volume) bool {
		if v.Name == vrConf.Volume {
			volume = v
			return true
		}
		return false
	}) {
		return fmt.Errorf("tried to install volume file into nonexistent volume: %v/%v", vrConf.Volume, res.Name)
	}
	inVolumeLoc := filepath.Join(volume.Mountpoint, vrConf.Path)
	data, err := os.ReadFile(res.Path)
	if err != nil {
		return err
	}
	mode := vrConf.Mode
	if mode == "" {
		mode = "0o755"
	}
	parsedMode, err := strconv.ParseInt(mode, 8, 32)
	if err != nil {
		return err
	}
	err = os.WriteFile(inVolumeLoc, bytes.NewBuffer(data).Bytes(), os.FileMode(parsedMode))
	if err != nil {
		return err
	}
	if vrConf.Owner != "" {
		uid, err := strconv.ParseInt(vrConf.Owner, 10, 32)
		if err != nil {
			return err
		}
		err = os.Chown(inVolumeLoc, int(uid), -1)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Materia) RemoveVolumeFile(ctx context.Context, parent *components.Component, res components.Resource) error {
	var vrConf *manifests.VolumeResourceConfig
	for _, vr := range parent.VolumeResources {
		if vr.Resource == res.Name {
			vrConf = &vr
		}
	}
	if vrConf == nil {
		return fmt.Errorf("tried to remove volume file for nonexistent volume resource: /%v", res.Name)
	}
	vrConf.Volume = fmt.Sprintf("systemd-%v", vrConf.Volume)
	volumes, err := m.Containers.ListVolumes(ctx)
	if err != nil {
		return err
	}
	var volume *containers.Volume
	if !slices.ContainsFunc(volumes, func(v *containers.Volume) bool {
		if v.Name == vrConf.Volume {
			volume = v
			return true
		}
		return false
	}) {
		return fmt.Errorf("tried to remove volume file into nonexistent volume: %v/%v", vrConf.Volume, res.Name)
	}
	inVolumeLoc := filepath.Join(volume.Mountpoint, vrConf.Path)
	return os.Remove(inVolumeLoc)
}

func (m *Materia) Clean(ctx context.Context) error {
	err := m.CompRepo.Clean()
	if err != nil {
		return err
	}
	err = m.SourceRepo.Clean()
	if err != nil {
		return err
	}
	return nil
}

func (m *Materia) CleanComponent(ctx context.Context, name string) error {
	comp, ok := m.Facts.InstalledComponents[name]
	if !ok {
		return errors.New("component not installed")
	}
	emptyVars := make(map[string]any)
	for _, r := range comp.Resources {
		err := m.executeAction(ctx, Action{
			Todo:    resToAction(r, "remove"),
			Parent:  comp,
			Payload: r,
		}, emptyVars)
		if err != nil {
			return err
		}
	}
	return m.CompRepo.RemoveComponent(comp)
}

func (m *Materia) PlanComponent(ctx context.Context, name string, roles []string) (*Plan, error) {
	if roles != nil {
		m.Facts.Roles = roles
	}
	if name != "" {
		m.Facts.AssignedComponents = []string{name}
	}
	m.Services = &services.PlannedServiceManager{}
	m.Facts.InstalledComponents = make(map[string]*components.Component)
	return m.Plan(ctx)
}

func (m *Materia) ValidateComponents(ctx context.Context) ([]string, error) {
	var invalidComps []string
	dcomps, err := m.CompRepo.ListComponentNames()
	if err != nil {
		return invalidComps, fmt.Errorf("can't get components from prefix: %w", err)
	}
	for _, name := range dcomps {
		_, err = m.CompRepo.GetComponent(name)
		if err != nil {
			log.Warn("component unable to be loaded", "component", name)
			invalidComps = append(invalidComps, name)
		}
	}

	return invalidComps, nil
}

func (m *Materia) PurgeComponenet(ctx context.Context, name string) error {
	return m.CompRepo.PurgeComponentByName(name)
}

func (m *Materia) SavePlan(p *Plan, outputfile string) error {
	path := filepath.Join(m.OutputDir, outputfile)
	planOutput := struct {
		Timestamp time.Time `toml:"timestamp"`
		Plan      []string  `toml:"plan"`
	}{
		Timestamp: time.Now(),
		Plan:      p.PrettyLines(),
	}

	// Create or truncate the output file
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create file %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	err = toml.NewEncoder(file).Encode(planOutput)
	if err != nil {
		return fmt.Errorf("failed to encode plan to TOML: %w", err)
	}
	return nil
}
