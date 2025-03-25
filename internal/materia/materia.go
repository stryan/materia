package materia

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"git.saintnet.tech/stryan/materia/internal/containers"
	"git.saintnet.tech/stryan/materia/internal/repository"
	"git.saintnet.tech/stryan/materia/internal/secrets"
	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	"git.saintnet.tech/stryan/materia/internal/secrets/mem"
	"git.saintnet.tech/stryan/materia/internal/services"
	"git.saintnet.tech/stryan/materia/internal/source"
	"git.saintnet.tech/stryan/materia/internal/source/file"
	"git.saintnet.tech/stryan/materia/internal/source/git"
	"github.com/charmbracelet/log"
)

type MacroMap func(map[string]interface{}) template.FuncMap

type Materia struct {
	Facts         *Facts
	Manifest      *MateriaManifest
	Services      services.Services
	PodmanConn    context.Context
	Containers    containers.Containers
	sm            secrets.SecretsManager
	source        source.Source
	CompRepo      repository.Repository
	DataRepo      repository.Repository
	QuadletRepo   repository.Repository
	ScriptRepo    repository.Repository
	ServiceRepo   repository.Repository
	SourceRepo    repository.Repository
	rootComponent *Component
	macros        MacroMap
	snippets      map[string]*Snippet
	debug         bool
}

func NewMateria(ctx context.Context, c *Config, sm services.Services, cm containers.Containers) (*Materia, error) {
	prefix := "/var/lib"
	destination := "/etc/containers/systemd/"
	servicePath := "/usr/local/lib/systemd/system/"
	scriptsPath := "/usr/local/bin"

	if c.User.Username != "root" {
		home := c.User.HomeDir
		var found bool
		conf, found := os.LookupEnv("XDG_CONFIG_HOME")
		if !found {
			destination = fmt.Sprintf("%v/.config/containers/systemd/", home)
		} else {
			destination = fmt.Sprintf("%v/containers/systemd/", conf)
		}
		datadir, found := os.LookupEnv("XDG_DATA_HOME")
		if !found {
			prefix = fmt.Sprintf("%v/.local/share", home)
			servicePath = fmt.Sprintf("%v/.local/share/systemd/user", home)
		} else {
			prefix = datadir
			servicePath = fmt.Sprintf("%v/systemd/user", datadir)
		}
	}
	if c.Prefix != "" {
		prefix = c.Prefix
	}
	if c.Destination != "" {
		destination = c.Destination
	}
	if c.Services != "" {
		servicePath = c.Services
	}

	sourcePath := filepath.Join(prefix, "materia", "source")
	if _, err := os.Stat(destination); os.IsNotExist(err) {
		return nil, fmt.Errorf("destination %v does not exist, setup manually", destination)
	}
	if _, err := os.Stat(scriptsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("scripts location %v does not exist, setup manually", scriptsPath)
	}
	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("services location %v does not exist, setup manually", servicePath)
	}

	err := os.Mkdir(filepath.Join(prefix, "materia"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating prefix: %w", err)
	}
	err = os.Mkdir(sourcePath, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating source repo: %w", err)
	}
	err = os.Mkdir(filepath.Join(prefix, "materia", "components"), 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("error creating components in prefix: %w", err)
	}

	var source source.Source
	parsedPath := strings.Split(c.SourceURL, "://")
	switch parsedPath[0] {
	case "git":
		source, err = git.NewGitSource(sourcePath, parsedPath[1], c.GitConfig)
		if err != nil {
			return nil, fmt.Errorf("invalid git source: %w", err)
		}
	case "file":
		source = file.NewFileSource(sourcePath, parsedPath[1])
	default:
		return nil, fmt.Errorf("invalid source: %v", parsedPath[0])
	}

	// Ensure local cache
	log.Info("updating configured source cache")
	err = source.Sync(ctx)
	if err != nil {
		return nil, fmt.Errorf("error syncing source: %w", err)
	}
	log.Info("pulling manifest")
	man, err := LoadMateriaManifest(filepath.Join(sourcePath, "MANIFEST.toml"))
	if err != nil {
		return nil, fmt.Errorf("error loading manifest: %w", err)
	}
	if err := man.Validate(); err != nil {
		return nil, err
	}

	log.Info("loading facts")
	compRepo := &repository.ComponentRepository{DataPrefix: filepath.Join(prefix, "materia", "components"), QuadletPrefix: destination}
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
		CompRepo:      compRepo,
		DataRepo:      &repository.FileRepository{Prefix: filepath.Join(prefix, "materia", "components")},
		QuadletRepo:   &repository.FileRepository{Prefix: destination},
		ScriptRepo:    &repository.FileRepository{Prefix: scriptsPath},
		ServiceRepo:   &repository.FileRepository{Prefix: servicePath},
		SourceRepo:    &repository.ComponentRepository{DataPrefix: filepath.Join(sourcePath, "components")},
		snippets:      snips,
		rootComponent: &Component{Name: "root"},
	}
	m.macros = func(vars map[string]interface{}) template.FuncMap {
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
			"m_dataDir": func(arg string) string {
				path, err := m.DataRepo.Get(ctx, arg)
				if err != nil {
					return "UNKNOWN"
				}
				return path
			},
			"m_facts": func(arg string) interface{} {
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
		conf.RepoPath = sourcePath
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
		s, err := v.toSnippet()
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

func (m *Materia) updateComponents(ctx context.Context) (map[string]*Component, error) {
	updatedComponents := make(map[string]*Component)

	// figure out ones to add
	var found []string
	sourcePaths, err := m.SourceRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting source components: %w", err)
	}
	var sourceComps []*Component
	for _, v := range sourcePaths {
		comp, err := NewComponentFromSource(v)
		if err != nil {
			return nil, fmt.Errorf("error creating component from source: %w", err)
		}
		sourceComps = append(sourceComps, comp)
	}
	currentComps := make(map[string]*Component)
	maps.Copy(currentComps, m.Facts.InstalledComponents)
	for _, v := range sourceComps {
		if !slices.Contains(m.Facts.AssignedComponents, v.Name) {
			// not assigned to host, skip
			continue
		}
		found = append(found, v.Name)
		_, ok := currentComps[v.Name]
		if !ok {
			v.State = StateFresh
		} else {
			v.State = StateMayNeedUpdate
			delete(currentComps, v.Name)
		}
		updatedComponents[v.Name] = v
	}
	for _, v := range currentComps {
		v.State = StateNeedRemoval
		updatedComponents[v.Name] = v
	}
	if len(found) != len(m.Facts.AssignedComponents) {
		log.Debugf("New Components: %v Assigned Components: %v", found, m.Facts.AssignedComponents)
		return nil, fmt.Errorf("not all assigned components were found for this host")
	}

	return updatedComponents, nil
}

func (m *Materia) calculateDiffs(ctx context.Context, updates map[string]*Component, plan *Plan) (map[string]*Component, error) {
	keys := sortedKeys(updates)
	needUpdate := false
	for _, k := range keys {
		v := updates[k]
		if err := v.Validate(); err != nil {
			return nil, err
		}
		switch v.State {
		case StateFresh:
			plan.Add(Action{
				Todo:   ActionInstallComponent,
				Parent: v,
			})
			vars := m.sm.Lookup(ctx, secrets.SecretFilter{
				Hostname:  m.Facts.Hostname,
				Roles:     m.Facts.Roles,
				Component: v.Name,
			})
			for _, r := range v.Resources {
				err := v.test(ctx, m.macros, vars)
				if err != nil {
					return nil, fmt.Errorf("missing variable for component: %w", err)
				}
				plan.Add(Action{
					Todo:    r.toAction("install"),
					Parent:  v,
					Payload: r,
				})
				needUpdate = true
			}
			if v.Scripted {
				plan.Add(Action{
					Todo:   ActionSetupComponent,
					Parent: v,
				})
			}
		case StateMayNeedUpdate:
			original, ok := m.Facts.InstalledComponents[v.Name]
			if !ok {
				return nil, fmt.Errorf("tried to update non-installed component: %v", v.Name)
			}
			resourceActions, err := original.diff(v, m.macros, m.sm.Lookup(ctx, secrets.SecretFilter{
				Hostname:  m.Facts.Hostname,
				Roles:     m.Facts.Roles,
				Component: v.Name,
			}))
			if err != nil {
				log.Debugf("error diffing components: L (%v) R (%v)", original, v)
				return nil, err
			}
			if len(resourceActions) != 0 {
				plan.Append(resourceActions)
				v.State = StateNeedUpdate
				needUpdate = true
			} else {
				v.State = StateOK
			}
		case StateStale, StateNeedRemoval:
			for _, r := range v.Resources {
				plan.Add(Action{
					Todo:    r.toAction("remove"),
					Parent:  v,
					Payload: r,
				})
			}
			if v.Scripted {
				plan.Add(Action{
					Todo:   ActionCleanupComponent,
					Parent: v,
				})
			}
			plan.Add(Action{
				Todo:   ActionRemoveComponent,
				Parent: v,
			})
			needUpdate = true
		case StateRemoved:
			continue
		case StateUnknown:
			return nil, errors.New("found unknown component")
		default:
			panic(fmt.Sprintf("unexpected main.ComponentLifecycle: %#v", v.State))
		}
	}
	if needUpdate {
		plan.Add(Action{
			Todo:   ActionReloadUnits,
			Parent: m.rootComponent,
		})
	}
	return updates, nil
}

func (m *Materia) calculateServiceDiffs(ctx context.Context, comps map[string]*Component, plan *Plan) error {
	keys := sortedKeys(comps)
	for _, v := range keys {
		c := comps[v]
		switch c.State {
		case StateFresh:
			// need to install all services
			if len(c.Services) != 0 {
				for _, s := range c.Services {
					plan.Add(Action{
						Todo:    ActionStartService,
						Parent:  c,
						Payload: s,
					})
				}
			}
		case StateNeedUpdate:
			// need to install all services
			if len(c.Services) != 0 {
				for _, s := range c.Services {
					plan.Add(Action{
						Todo:    ActionRestartService,
						Parent:  c,
						Payload: s,
					})
				}
			}
		case StateOK:
			modified := false
			if len(c.Services) != 0 {
				for _, s := range c.Services {
					state, err := m.Services.Get(ctx, s.Name)
					if err != nil {
						return err
					}
					if state.State != "active" {
						plan.Add(Action{
							Todo:    ActionStartService,
							Parent:  c,
							Payload: s,
						})
						modified = true
					}
				}
			}
			if modified {
				c.State = StateNeedUpdate
			}
		case StateRemoved:
			// need to stop all services
			if len(c.Services) != 0 {
				for _, s := range c.Services {
					plan.Add(Action{
						Todo:    ActionStopService,
						Parent:  c,
						Payload: s,
					})
				}
			}
		default:
			continue
		}
	}
	return nil
}

func (m *Materia) modifyService(ctx context.Context, command Action) error {
	if err := command.Validate(); err != nil {
		return err
	}
	var res Resource
	if command.Todo != ActionReloadUnits {
		res = command.Payload
		if err := res.Validate(); err != nil {
			return fmt.Errorf("invalid resource when modifying service: %w", err)
		}

		if res.Kind != ResourceTypeService {
			return errors.New("attempted to modify a non service resource")
		}
	}
	var cmd services.ServiceAction
	switch command.Todo {
	case ActionStartService:
		cmd = services.ServiceStart
		log.Info("starting service", "unit", res.Name)
	case ActionStopService:
		log.Info("stopping service", "unit", res.Name)
		cmd = services.ServiceStop
	case ActionRestartService:
		log.Info("restarting service", "unit", res.Name)
		cmd = services.ServiceRestart
	case ActionReloadUnits:
		log.Info("reloading units")
		cmd = services.ServiceReload
	default:
		return errors.New("invalid service command")
	}
	return m.Services.Apply(ctx, res.Name, cmd)
}

func (m *Materia) Plan(ctx context.Context) (*Plan, error) {
	plan := NewPlan(m.Facts)
	var err error

	// Determine union of existing and new components
	if len(m.Facts.InstalledComponents) == 0 && len(m.Facts.AssignedComponents) == 0 {
		return plan, nil
	}

	var newComponents map[string]*Component
	log.Debug("calculating component differences")
	if newComponents, err = m.updateComponents(ctx); err != nil {
		return plan, fmt.Errorf("error determining components: %w", err)
	}
	// Determine diff actions
	log.Debug("calculating resource differences")
	finalComponents, err := m.calculateDiffs(ctx, newComponents, plan)
	if err != nil {
		return plan, fmt.Errorf("error calculating diffs: %w", err)
	}

	// determine service actions

	log.Debug("calculating service differences")
	err = m.calculateServiceDiffs(ctx, finalComponents, plan)
	if err != nil {
		return plan, fmt.Errorf("error calculating service actions: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("generated invalid plan: %w", err)
	}
	var installing, removing, updating, ok []string
	keys := sortedKeys(finalComponents)
	for _, k := range keys {
		v := finalComponents[k]
		switch v.State {
		case StateFresh:
			installing = append(installing, v.Name)
			log.Debug("fresh:", "component", v.Name)
		case StateNeedUpdate:
			updating = append(updating, v.Name)
			log.Debug("updating:", "component", v.Name)
		case StateMayNeedUpdate:
			log.Warn("component still listed as may need update", "component", v.Name)
		case StateNeedRemoval:
			removing = append(removing, v.Name)
			log.Debug("remove:", "component", v.Name)
		case StateOK:
			ok = append(ok, v.Name)
			log.Debug("ok:", "component", v.Name)
		case StateRemoved:
			log.Debug("removed:", "component", v.Name)
		case StateStale:
			log.Debug("stale:", "component", v.Name)
		case StateUnknown:
			log.Debug("unknown:", "component", v.Name)
		default:
			panic(fmt.Sprintf("unexpected main.ComponentLifecycle: %#v", v.State))
		}
	}

	log.Debug("installing components", "installing", installing)
	log.Debug("removing components", "removing", removing)
	log.Debug("updating components", "updating", updating)
	log.Debug("unchanged components", "unchanged", ok)
	log.Debug("plan", "plan", plan)

	return plan, nil
}

func (m *Materia) Execute(ctx context.Context, plan *Plan) error {
	if plan.Empty() {
		return nil
	}
	serviceActions := []Action{}
	// Template and install resources
	for _, v := range plan.Steps() {
		vars := make(map[string]interface{})
		if err := v.Validate(); err != nil {
			return err
		}
		vaultVars := m.sm.Lookup(ctx, secrets.SecretFilter{
			Hostname:  m.Facts.Hostname,
			Roles:     m.Facts.Roles,
			Component: v.Parent.Name,
		})
		maps.Copy(vars, v.Parent.Defaults)
		maps.Copy(vars, vaultVars)

		switch v.Todo {
		case ActionInstallComponent:
			if err := m.CompRepo.Install(ctx, v.Parent.Name, nil); err != nil {
				return err
			}
		case ActionInstallFile, ActionUpdateFile:
			resourceData, err := v.Payload.execute(m.macros, vars)
			if err != nil {
				return err
			}
			if err := m.DataRepo.Install(ctx, filepath.Join(v.Parent.Name, v.Payload.Name), resourceData); err != nil {
				return err
			}
		case ActionInstallQuadlet, ActionUpdateQuadlet:
			resourceData, err := v.Payload.execute(m.macros, vars)
			if err != nil {
				return err
			}
			if err := m.QuadletRepo.Install(ctx, filepath.Join(v.Parent.Name, v.Payload.Name), resourceData); err != nil {
				return err
			}
		case ActionInstallScript, ActionUpdateScript:
			resourceData, err := v.Payload.execute(m.macros, vars)
			if err != nil {
				return err
			}
			if err := m.DataRepo.Install(ctx, filepath.Join(v.Parent.Name, v.Payload.Name), resourceData); err != nil {
				return err
			}
			if err := m.ScriptRepo.Install(ctx, v.Payload.Name, resourceData); err != nil {
				return err
			}
		case ActionInstallService, ActionUpdateService:
			resourceData, err := v.Payload.execute(m.macros, vars)
			if err != nil {
				return err
			}
			if err := m.DataRepo.Install(ctx, filepath.Join(v.Parent.Name, v.Payload.Name), resourceData); err != nil {
				return err
			}
			if err := m.ServiceRepo.Install(ctx, v.Payload.Name, resourceData); err != nil {
				return err
			}
		case ActionInstallComponentScript, ActionUpdateComponentScript:
			resourceData, err := v.Payload.execute(m.macros, vars)
			if err != nil {
				return err
			}
			if err := m.DataRepo.Install(ctx, filepath.Join(v.Parent.Name, v.Payload.Name), resourceData); err != nil {
				return err
			}
		case ActionRemoveFile:
			if err := m.DataRepo.Remove(ctx, filepath.Join(v.Parent.Name, v.Payload.Name)); err != nil {
				return err
			}
		case ActionRemoveQuadlet:
			if err := m.QuadletRepo.Remove(ctx, filepath.Join(v.Parent.Name, v.Payload.Name)); err != nil {
				return err
			}
		case ActionRemoveScript:
			if err := m.DataRepo.Remove(ctx, filepath.Join(v.Parent.Name, v.Payload.Name)); err != nil {
				return err
			}
			if err := m.ScriptRepo.Remove(ctx, v.Payload.Name); err != nil {
				return err
			}
		case ActionRemoveService:
			if err := m.DataRepo.Remove(ctx, filepath.Join(v.Parent.Name, v.Payload.Name)); err != nil {
				return err
			}
			if err := m.ServiceRepo.Remove(ctx, v.Payload.Name); err != nil {
				return err
			}
		case ActionRemoveComponentScript:
			if err := m.DataRepo.Remove(ctx, filepath.Join(v.Parent.Name, v.Payload.Name)); err != nil {
				return err
			}
		case ActionRemoveComponent:
			if err := m.CompRepo.Remove(ctx, v.Parent.Name); err != nil {
				return err
			}

		case ActionCleanupComponent:
			path, err := m.DataRepo.Get(ctx, v.Parent.Name)
			if err != nil {
				return err
			}
			cmd := exec.Command(fmt.Sprintf("%v/cleanup.sh", path))

			cmd.Dir = path
			err = cmd.Run()
			if err != nil {
				return err
			}
		case ActionEnsureVolume:
			service := strings.TrimSuffix(v.Payload.Name, ".volume")
			err := m.modifyService(ctx, Action{
				Todo:   ActionStartService,
				Parent: v.Parent,
				Payload: Resource{
					Name: fmt.Sprintf("%v-volume.service", service),
					Kind: ResourceTypeService,
				},
			})
			if err != nil {
				return err
			}
		case ActionInstallVolumeResource:
			resourceData, err := v.Payload.execute(m.macros, vars)
			if err != nil {
				return err
			}
			if err := m.DataRepo.Install(ctx, filepath.Join(v.Parent.Name, v.Payload.Name), resourceData); err != nil {
				return err
			}
			if err := m.InstallVolumeFile(ctx, v.Parent, v.Payload); err != nil {
				return err
			}
		case ActionUpdateVolumeResource:
			resourceData, err := v.Payload.execute(m.macros, vars)
			if err != nil {
				return err
			}
			if err := m.DataRepo.Install(ctx, filepath.Join(v.Parent.Name, v.Payload.Name), resourceData); err != nil {
				return err
			}
			if err := m.InstallVolumeFile(ctx, v.Parent, v.Payload); err != nil {
				return err
			}
		case ActionRemoveVolumeResource:
			if err := m.DataRepo.Remove(ctx, filepath.Join(v.Parent.Name, v.Payload.Name)); err != nil {
				return err
			}
			if err := m.RemoveVolumeFile(ctx, v.Parent, v.Payload); err != nil {
				return err
			}
		case ActionSetupComponent:
			path, err := m.DataRepo.Get(ctx, v.Parent.Name)
			if err != nil {
				return err
			}
			cmd := exec.Command(fmt.Sprintf("%v/setup.sh", path))
			cmd.Dir = path
			err = cmd.Run()
			if err != nil {
				return err
			}
		case ActionStartService, ActionStopService, ActionRestartService:
			err := m.modifyService(ctx, v)
			if err != nil {
				return err
			}
			serviceActions = append(serviceActions, v)
		case ActionReloadUnits:
			err := m.modifyService(ctx, v)
			if err != nil {
				return err
			}
		default:
		}
	}

	// verify services
	for _, v := range serviceActions {
		serv, err := m.Services.Get(ctx, v.Payload.Name)
		if err != nil {
			return err
		}
		switch v.Todo {
		case ActionRestartService, ActionStartService:
			if serv.State != "active" {
				log.Warn("service failed to start/restart", "service", serv.Name, "state", serv.State)
			}
		case ActionStopService:
			if serv.State != "inactive" {
				log.Warn("service failed to stop", "service", serv.Name, "state", serv.State)
			}
		default:
			return errors.New("unknown service action state")
		}
	}
	return nil
}

func (m *Materia) InstallVolumeFile(ctx context.Context, parent *Component, res Resource) error {
	var vrConf *VolumeResourceConfig
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

func (m *Materia) RemoveVolumeFile(ctx context.Context, parent *Component, res Resource) error {
	var vrConf *VolumeResourceConfig
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
	err := m.CompRepo.Clean(ctx)
	if err != nil {
		return err
	}
	err = m.DataRepo.Clean(ctx)
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
	return m.CompRepo.Remove(ctx, comp.Name)
}

func (m *Materia) ValidateComponent(ctx context.Context, name string, roles []string) (*Plan, error) {
	if roles != nil {
		m.Facts.Roles = roles
	}
	if name != "" {
		m.Facts.AssignedComponents = []string{name}
	}
	m.Facts.InstalledComponents = make(map[string]*Component)
	return m.Plan(ctx)
}

func sortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	keys := make([]K, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
