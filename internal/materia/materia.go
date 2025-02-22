package materia

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"text/template"

	"git.saintnet.tech/stryan/materia/internal/secrets"
	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	"git.saintnet.tech/stryan/materia/internal/secrets/mem"
	"git.saintnet.tech/stryan/materia/internal/source"
	"git.saintnet.tech/stryan/materia/internal/source/file"
	"git.saintnet.tech/stryan/materia/internal/source/git"
	"github.com/charmbracelet/log"
)

type Materia struct {
	Facts         *Facts
	Manifest      *MateriaManifest
	Services      Services
	PodmanConn    context.Context
	Containers    Containers
	sm            secrets.SecretsManager
	source        source.Source
	files         Repository
	rootComponent *Component
	macros        func(map[string]interface{}) template.FuncMap
	snippets      map[string]*Snippet
	debug         bool
}

func NewMateria(ctx context.Context, c *Config, sm Services, cm Containers) (*Materia, error) {
	prefix := "/var/lib"
	destination := "/etc/containers/systemd/"
	services := "/etc/systemd/system/"

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
			services = fmt.Sprintf("%v/.local/share/systemd/user", home)
		} else {
			prefix = datadir
			services = fmt.Sprintf("%v/systemd/user", datadir)
		}
	}
	if c.Prefix != "" {
		prefix = c.Prefix
	}
	if c.Destination != "" {
		destination = c.Destination
	}
	if c.Services != "" {
		services = c.Services
	}

	var source source.Source
	sourcePath := filepath.Join(prefix, "materia", "source")
	parsedPath := strings.Split(c.SourceURL, "://")
	switch parsedPath[0] {
	case "git":
		source = git.NewGitSource(sourcePath, parsedPath[1], c.PrivateKey)
	case "file":
		source = file.NewFileSource(sourcePath, parsedPath[1])
	default:
		return nil, fmt.Errorf("invalid source: %v", parsedPath[0])
	}
	files := NewFileRepository(prefix, destination, filepath.Join(prefix, "materia", "components"), services, sourcePath, c.Debug)
	snips := make(map[string]*Snippet)
	defaultSnippets := loadDefaultSnippets()
	for _, v := range defaultSnippets {
		snips[v.Name] = v
	}

	man, facts, err := NewFacts(ctx, c, source, files)
	if err != nil {
		return nil, fmt.Errorf("error generating facts: %w", err)
	}

	m := &Materia{
		Services:      sm,
		Containers:    cm,
		Facts:         facts,
		Manifest:      man,
		source:        source,
		debug:         c.Debug,
		files:         files,
		snippets:      snips,
		rootComponent: &Component{Name: "root"},
	}
	m.macros = func(vars map[string]interface{}) template.FuncMap {
		return template.FuncMap{
			"m_deps": func(arg string) string {
				switch arg {
				case "after":
					if res, ok := vars["After"]; ok {
						return res.(string)
					} else {
						return "local-fs.target network.target"
					}
				case "wants":
					if res, ok := vars["Wants"]; ok {
						return res.(string)
					} else {
						return "local-fs.target network.target"
					}
				case "requires":
					if res, ok := vars["Requires"]; ok {
						return res.(string)
					} else {
						return "local-fs.target network.target"
					}
				default:
					return "ERR_BAD_DEFAULT"
				}
			},
			"m_dataDir": func(arg string) string {
				return m.files.DataPath(arg)
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
	return m, nil
}

func (m *Materia) Close() {
	m.Services.Close()
	m.Containers.Close()
}

func (m *Materia) Prepare(ctx context.Context) error {
	if err := m.files.Setup(ctx); err != nil {
		return fmt.Errorf("error setting up files: %w", err)
	}
	var err error
	// Ensure local cache
	log.Info("updating configured source cache")
	err = m.source.Sync(ctx)
	if err != nil {
		return fmt.Errorf("error syncing source: %w", err)
	}
	switch m.Manifest.Secrets {
	case "age":
		conf, ok := m.Manifest.SecretsConfig.(age.Config)
		if !ok {
			return errors.New("tried to create an age secrets manager but config was not for age")
		}
		m.sm, err = age.NewAgeStore(age.Config{
			IdentPath: conf.IdentPath,
			RepoPath:  m.files.SourcePath(),
		})
		if err != nil {
			return fmt.Errorf("error creating age store: %w", err)
		}

	case "mem":
		m.sm = mem.NewMemoryManager()
	default:
		m.sm = mem.NewMemoryManager()
	}
	for _, v := range m.Manifest.Snippets {
		s, err := v.toSnippet()
		if err != nil {
			return err
		}
		m.snippets[s.Name] = s
	}
	return nil
}

func (m *Materia) newDetermineComponents(ctx context.Context) (map[string]*Component, map[string]*Component, error) {
	currentComponents := make(map[string]*Component)
	updatedComponents := make(map[string]*Component)

	comps, err := m.files.GetAllInstalledComponents(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting installed components: %w", err)
	}
	log.Debug(comps)
	for _, v := range comps {
		v.State = StateStale
		currentComponents[v.Name] = v
	}
	// figure out ones to add
	var found []string
	// TODO figure out role assignments

	newComps, err := m.files.GetAllSourceComponents(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting source components: %w", err)
	}
	for _, v := range newComps {
		if !slices.Contains(m.Facts.Components, v.Name) {
			continue
		}
		found = append(found, v.Name)
		existing, ok := currentComponents[v.Name]
		if !ok {
			v.State = StateFresh
			currentComponents[v.Name] = v
		} else {
			v.State = StateCanidate
			updatedComponents[v.Name] = v
			existing.State = StateMayNeedUpdate
			existing.Defaults = v.Defaults
			currentComponents[v.Name] = existing
		}
	}
	for _, v := range currentComponents {
		if v.State == StateStale {
			// exists on disk but not in source, remove
			v.State = StateNeedRemoval
		}
	}
	if len(found) != len(m.Facts.Components) {
		log.Debugf("New Components: %v Assigned Components: %v", found, m.Facts.Components)
		return nil, nil, fmt.Errorf("not all assigned components were found for this host")
	}

	return currentComponents, updatedComponents, nil
}

func (m *Materia) calculateDiffs(ctx context.Context, currentComponents, newComponents map[string]*Component) ([]Action, error) {
	var actions []Action
	keys := sortedKeys(currentComponents)
	for _, k := range keys {
		v := currentComponents[k]
		if err := v.Validate(); err != nil {
			return actions, err
		}
		switch v.State {
		case StateFresh:
			actions = append(actions, Action{
				Todo:   ActionInstallComponent,
				Parent: v,
			})

			for _, r := range v.Resources {
				err := v.test(ctx, m.macros, m.sm.Lookup(ctx, secrets.SecretFilter{
					Hostname:  m.Facts.Hostname,
					Roles:     m.Facts.Roles,
					Component: v.Name,
				}))
				if err != nil {
					return actions, fmt.Errorf("missing variable for component: %w", err)
				}
				actions = append(actions, Action{
					Todo:    ActionInstallResource,
					Parent:  v,
					Payload: r,
				})
			}
			if v.Scripted {
				actions = append(actions, Action{
					Todo:   ActionSetupComponent,
					Parent: v,
				})
			}
		case StateMayNeedUpdate:
			candidate, ok := newComponents[v.Name]
			if !ok {
				return actions, errors.New("tried to replace component with nonexistent candidate")
			}
			resourceActions, err := v.diff(candidate, m.macros, m.sm.Lookup(ctx, secrets.SecretFilter{
				Hostname:  m.Facts.Hostname,
				Roles:     m.Facts.Roles,
				Component: v.Name,
			}))
			if err != nil {
				log.Debugf("error diffing components: L (%v) R (%v)", v, candidate)
				return actions, err
			}
			if len(resourceActions) != 0 {
				actions = append(actions, resourceActions...)
				v.State = StateNeedUpdate
			} else {
				v.State = StateOK
			}
		case StateStale, StateNeedRemoval:
			if v.Scripted {
				actions = append(actions, Action{
					Todo:   ActionCleanupComponent,
					Parent: v,
				})
			}
			actions = append(actions, Action{
				Todo:   ActionRemoveComponent,
				Parent: v,
			})
		case StateRemoved:
			continue
		case StateUnknown:
			return actions, errors.New("found unknown component")
		default:
			panic(fmt.Sprintf("unexpected main.ComponentLifecycle: %#v", v.State))
		}
	}
	return actions, nil
}

func (m *Materia) calculateVolDiffs(ctx context.Context, _ secrets.SecretsManager, components map[string]*Component) ([]Action, error) {
	var actions []Action
	keys := sortedKeys(components)
	for _, k := range keys {
		v := components[k]
		if err := v.Validate(); err != nil {
			return actions, err
		}
		for _, r := range v.Resources {
			if err := r.Validate(); err != nil {
				return actions, err
			}
			if r.Kind == ResourceTypeVolumeFile {
				splitp := strings.Split(r.Path, ":")
				if len(splitp) != 2 {
					return actions, fmt.Errorf("invalid volume path name: %v", r.Path)
				}
				volName := splitp[0]
				volResource := splitp[1]
				// ensure volume exists
				err := m.Services.Start(ctx, volName)
				if err != nil {
					return actions, err
				}
				resp, err := m.Containers.InspectVolume(volName)
				if err != nil {
					return actions, err
				}
				inVolLoc := fmt.Sprintf("%v/%v", resp.Mountpoint, volResource)
				if _, err := os.Stat(inVolLoc); errors.Is(err, os.ErrNotExist) {
					// VolumeResource does not exist
					finalPayload := r
					finalPayload.Path = inVolLoc
					actions = append(actions, Action{
						Todo:    ActionInstallVolumeResource,
						Parent:  v,
						Payload: finalPayload,
					})
				} else if err != nil {
					return actions, err
				}
				// TODO diff here
				log.Info("TODO diff")
				finalPayload := r
				finalPayload.Path = inVolLoc
				actions = append(actions, Action{
					Todo:    ActionInstallVolumeResource,
					Parent:  v,
					Payload: finalPayload,
				})
			}
		}
	}

	return actions, nil
}

func (m *Materia) calculateServiceDiffs(ctx context.Context, comps map[string]*Component) ([]Action, error) {
	var actions []Action
	keys := sortedKeys(comps)
	for _, v := range keys {
		c := comps[v]
		switch c.State {
		case StateFresh:
			// need to install all services
			if len(c.Services) != 0 {
				for _, s := range c.Services {
					actions = append(actions, Action{
						Todo:    ActionStartService,
						Parent:  c,
						Payload: s,
					})
				}
			} else {
				for _, r := range c.Resources {
					if r.Kind == ResourceTypeContainer || r.Kind == ResourceTypePod {
						serv, err := getServicefromResource(r)
						if err != nil {
							return actions, err
						}
						actions = append(actions, Action{
							Todo:    ActionStartService,
							Parent:  c,
							Payload: serv,
						})
					}
				}
			}
		case StateNeedUpdate:
			// need to install all services
			if len(c.Services) != 0 {
				for _, s := range c.Services {
					actions = append(actions, Action{
						Todo:    ActionRestartService,
						Parent:  c,
						Payload: s,
					})
				}
			} else {
				for _, r := range c.Resources {
					if r.Kind == ResourceTypeContainer || r.Kind == ResourceTypePod {
						serv, err := getServicefromResource(r)
						if err != nil {
							return actions, err
						}
						actions = append(actions, Action{
							Todo:    ActionRestartService,
							Parent:  c,
							Payload: serv,
						})
					}
				}
			}
		case StateOK:
			modified := false
			if len(c.Services) != 0 {
				for _, s := range c.Services {
					state, err := m.Services.Get(ctx, s.Name)
					if err != nil {
						return actions, err
					}
					if state.State != "active" {
						actions = append(actions, Action{
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
					actions = append(actions, Action{
						Todo:    ActionStopService,
						Parent:  c,
						Payload: s,
					})
				}
			} else {
				for _, r := range c.Resources {
					if r.Kind == ResourceTypeContainer || r.Kind == ResourceTypePod {
						serv, err := getServicefromResource(r)
						if err != nil {
							return actions, err
						}
						actions = append(actions, Action{
							Todo:    ActionStopService,
							Parent:  c,
							Payload: serv,
						})
					}
				}
			}
		default:
			continue
		}
	}
	return actions, nil
}

func getServicefromResource(serv Resource) (Resource, error) {
	var res Resource
	switch serv.Kind {
	case ResourceTypeContainer:
		servicename, found := strings.CutSuffix(serv.Name, ".container")
		if !found {
			return res, fmt.Errorf("invalid container name for service: %v", serv.Name)
		}
		res = Resource{
			Name: fmt.Sprintf("%v.service", servicename),
			Kind: ResourceTypeService,
		}
	case ResourceTypePod:
		podname, found := strings.CutSuffix(serv.Name, ".pod")
		if !found {
			return res, fmt.Errorf("invalid pod name %v", serv.Name)
		}
		res = Resource{
			Name: fmt.Sprintf("%v-pod.service", podname),
			Kind: ResourceTypeService,
		}
	case ResourceTypeService:
		return serv, nil
	default:
		return res, errors.New("tried to convert a non container or pod to a service")
	}
	return res, nil
}

func (m *Materia) modifyService(ctx context.Context, command Action) error {
	var err error
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
	switch command.Todo {
	case ActionStartService:
		log.Info("starting service", "unit", res.Name)
		err = m.Services.Start(ctx, res.Name)
	case ActionStopService:
		log.Info("stopping service", "unit", res.Name)
		err = m.Services.Stop(ctx, res.Name)

	case ActionRestartService:
		log.Info("restarting service", "unit", res.Name)
		err = m.Services.Restart(ctx, res.Name)

	case ActionReloadUnits:
		log.Info("reloading units")
		err = m.Services.Reload(ctx)

	default:
		return errors.New("invalid service command")
	}
	return err
}

func (m *Materia) Plan(ctx context.Context) ([]Action, error) {
	var actions []Action
	var err error

	// Determine assigned components
	// Determine existing components
	var components map[string]*Component
	var newComponents map[string]*Component
	if components, newComponents, err = m.newDetermineComponents(ctx); err != nil {
		return actions, fmt.Errorf("error determining components: %w", err)
	}
	if len(components) == 0 && len(newComponents) == 0 {
		return actions, nil
	}

	// Determine diff actions
	diffActions, err := m.calculateDiffs(ctx, components, newComponents)
	if err != nil {
		return actions, fmt.Errorf("error calculating diffs: %w", err)
	}

	// determine volume actions
	volResourceActions, err := m.calculateVolDiffs(ctx, m.sm, components)
	if err != nil {
		return actions, fmt.Errorf("error calculating volume diffs: %w", err)
	}
	// determine service actions
	serviceActions, err := m.calculateServiceDiffs(ctx, components)
	if err != nil {
		return actions, fmt.Errorf("error calculating service actions: %w", err)
	}
	log.Debug("component actions")
	var installing, removing, updating, ok []string
	keys := sortedKeys(components)
	for _, k := range keys {
		v := components[k]
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

	log.Info("installing components", "installing", installing)
	log.Info("removing components", "removing", removing)
	log.Info("updating components", "updating", updating)
	log.Info("unchanged components", "unchanged", ok)
	log.Debug("diff actions", "diffActions", diffActions)
	log.Debug("volume actions", "volResourceActions", volResourceActions)
	log.Debug("service actions", "serviceActions", serviceActions)
	actions = append(diffActions, volResourceActions...)
	actions = append(actions, serviceActions...)
	return actions, nil
}

func (m *Materia) Execute(ctx context.Context, plan []Action) error {
	if len(plan) == 0 {
		return nil
	}
	// Template and install resources
	resourceChanged := false
	for _, v := range plan {
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
			if err := m.files.InstallComponent(v.Parent, m.sm); err != nil {
				return err
			}
			resourceChanged = true
		case ActionInstallResource:
			if err := m.files.InstallResource(ctx, v.Parent, v.Payload, m.macros, vars); err != nil {
				return err
			}

			resourceChanged = true
		case ActionUpdateResource:
			if err := m.files.InstallResource(ctx, v.Parent, v.Payload, m.macros, vars); err != nil {
				return err
			}

			resourceChanged = true
		case ActionRemoveComponent:
			if err := m.files.RemoveComponent(v.Parent, m.sm); err != nil {
				return err
			}

			resourceChanged = true
		case ActionRemoveResource:
			if err := m.files.RemoveResource(v.Parent, v.Payload, m.sm); err != nil {
				return err
			}

			resourceChanged = true
		case ActionCleanupComponent:
			cmd := exec.Command(fmt.Sprintf("%v/cleanup.sh", m.files.DataPath(v.Parent.Name)))
			cmd.Dir = m.files.DataPath(v.Parent.Name)
			err := cmd.Run()
			if err != nil {
				return err
			}
		default:
		}
	}

	// If any resource actions were taken, daemon-reload
	if resourceChanged {
		err := m.modifyService(ctx, Action{
			Todo:   ActionReloadUnits,
			Parent: m.rootComponent,
		})
		if err != nil {
			return err
		}
	}
	// Anything that needs updated unit list but pre-service starting
	for _, v := range plan {
		switch v.Todo {
		case ActionInstallVolumeResource:
			err := m.files.InstallResource(ctx, v.Parent, v.Payload, m.macros, m.sm.Lookup(ctx, secrets.SecretFilter{
				Hostname:  m.Facts.Hostname,
				Roles:     m.Facts.Roles,
				Component: v.Parent.Name,
			}))
			if err != nil {
				return err
			}
		case ActionSetupComponent:
			cmd := exec.Command(fmt.Sprintf("%v/setup.sh", m.files.DataPath(v.Parent.Name)))
			cmd.Dir = m.files.DataPath(v.Parent.Name)
			err := cmd.Run()
			if err != nil {
				return err
			}

		}
	}
	// Ensure volumes and volume resources
	// start/stop services
	serviceActions := []Action{}
	for _, v := range plan {
		if v.Todo == ActionStartService || v.Todo == ActionStopService || v.Todo == ActionRestartService {
			err := m.modifyService(ctx, v)
			if err != nil {
				return err
			}
			serviceActions = append(serviceActions, v)
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

func (m *Materia) Clean(ctx context.Context) error {
	return m.files.Clean(ctx)
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
