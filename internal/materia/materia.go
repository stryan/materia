package materia

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"html/template"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/secrets"
	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	"git.saintnet.tech/stryan/materia/internal/secrets/mem"
	"git.saintnet.tech/stryan/materia/internal/source"
	"git.saintnet.tech/stryan/materia/internal/source/file"
	"git.saintnet.tech/stryan/materia/internal/source/git"
	"github.com/charmbracelet/log"
)

type Materia struct {
	Services          Services
	PodmanConn        context.Context
	Containers        Containers
	sm                secrets.SecretsManager
	source            source.Source
	files             Repository
	rootComponent     *Component
	templateFunctions func(map[string]interface{}) template.FuncMap
	debug             bool
}

func NewMateria(ctx context.Context, c *Config, sm Services, cm Containers) (*Materia, error) {
	prefix := "/var/lib"
	destination := "/etc/containers/systemd/"

	if c.User.Username != "root" {
		home := c.User.HomeDir
		var found bool
		conf, found := os.LookupEnv("XDG_CONFIG_HOME")
		if !found {
			destination = fmt.Sprintf("%v/.config/containers/systemd/", home)
		} else {
			destination = fmt.Sprintf("%v/containers/systemd/", conf)
		}
		prefix, found = os.LookupEnv("XDG_DATA_HOME")
		if !found {
			prefix = fmt.Sprintf("%v/.local/share", home)
		}
	}
	if c.Prefix != "" {
		prefix = c.Prefix
	}
	if c.Destination != "" {
		destination = c.Destination
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
		return nil, errors.New("invalid source")
	}
	m := &Materia{
		Services:      sm,
		Containers:    cm,
		source:        source,
		debug:         c.Debug,
		files:         NewFileRepository(prefix, destination, filepath.Join(prefix, "materia", "components"), sourcePath, c.Debug),
		rootComponent: &Component{Name: "root"},
	}
	m.templateFunctions = func(vars map[string]interface{}) template.FuncMap {
		return template.FuncMap{
			"materia_defaults": func(arg string) string {
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
			"materia_auto_update": func(arg string) string {
				return fmt.Sprintf("Label=io.containers.autoupdate=%v", arg)
			},
			"quadletDataDir": func(arg string) string {
				return m.files.DataPath(arg)
			},
			"exists": func(arg string) bool {
				_, ok := vars[arg]
				return ok
			},
		}
	}
	return m, nil
}

func (m *Materia) Close() {
	m.Services.Close()
	m.Containers.Close()
	// TODO do something with closing the podman context here
}

func (m *Materia) Prepare(ctx context.Context, man *MateriaManifest) error {
	if err := man.Validate(); err != nil {
		return fmt.Errorf("invalid manifest %w", err)
	}
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
	switch man.Secrets {
	case "age":
		conf, ok := man.SecretsConfig.(age.Config)
		if !ok {
			return errors.New("tried to create an age secrets manager but config was not for age")
		}
		// TODO IdentPath needs to be customized
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

	return nil
}

func (m *Materia) newDetermineComponents(ctx context.Context, man *MateriaManifest, facts *Facts) (map[string]*Component, map[string]*Component, error) {
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
	var whitelist []string
	var found []string
	// TODO figure out role assignments
	host, ok := man.Hosts["all"]
	if ok {
		whitelist = append(whitelist, host.Components...)
	}
	host, ok = man.Hosts[facts.Hostname]
	if ok {
		whitelist = append(whitelist, host.Components...)
	}

	newComps, err := m.files.GetAllSourceComponents(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting source components: %w", err)
	}
	for _, v := range newComps {
		if !slices.Contains(whitelist, v.Name) {
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
	if len(found) != len(whitelist) {
		return nil, nil, fmt.Errorf("not all assigned components were found for this host")
	}

	return currentComponents, updatedComponents, nil
}

func (m *Materia) calculateDiffs(ctx context.Context, f *Facts, sm secrets.SecretsManager, currentComponents, newComponents map[string]*Component) ([]Action, error) {
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
				err := v.test(ctx, m.templateFunctions, sm.Lookup(ctx, secrets.SecretFilter{
					Hostname:  f.Hostname,
					Role:      f.Role,
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
		case StateMayNeedUpdate:
			candidate, ok := newComponents[v.Name]
			if !ok {
				return actions, errors.New("tried to replace component with nonexistent candidate")
			}
			resourceActions, err := v.diff(candidate, m.templateFunctions, sm.Lookup(ctx, secrets.SecretFilter{
				Hostname:  f.Hostname,
				Role:      f.Role,
				Component: v.Name,
			}))
			if err != nil {
				log.Debugf("error diffing components: L (%v) R (%v)", v, candidate)
				return actions, err
			}
			if len(resourceActions) != 0 {
				actions = append(actions, resourceActions...)
			} else {
				v.State = StateOK
			}
		case StateStale, StateNeedRemoval:
			actions = append(actions, Action{
				Todo:   ActionRemoveComponent,
				Parent: v,
			})
		case StateOK, StateRemoved:
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

// TODO refactor this into a per resource thing
func getServicesFromResources(servs []Resource) []Resource {
	services := []Resource{}
	// if there's any pods in the list, use them instead of raw container files
	hasPods := slices.ContainsFunc(servs, func(r Resource) bool { return r.Kind == ResourceTypePod })
	if hasPods {
		for _, s := range servs {
			if s.Kind == ResourceTypePod {
				podname, found := strings.CutSuffix(s.Name, ".pod")
				if !found {
					log.Warn("invalid pod name", "raw_name", s.Name)
				}
				services = append(services, Resource{
					Name: fmt.Sprintf("%v-pod.service", podname),
					Kind: ResourceTypeService,
				})
			}
		}
	} else {
		for _, s := range servs {
			if s.Kind == ResourceTypeContainer {
				servicename, found := strings.CutSuffix(s.Name, ".container")
				if !found {
					log.Warn("invalid service name", "raw_name", s.Name)
				}
				services = append(services, Resource{
					Name: fmt.Sprintf("%v.service", servicename),
					Kind: ResourceTypeService,
				})
			}
		}
	}
	return services
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
		if err != nil {
			log.Warnf("error starting service: %v", err)
		}
	case ActionStopService:
		log.Info("stopping service", "unit", res.Name)
		err = m.Services.Stop(ctx, res.Name)
		if err != nil {
			log.Warnf("error starting service: %v", err)
		}

	case ActionRestartService:
		log.Info("restarting service", "unit", res.Name)
		err = m.Services.Restart(ctx, res.Name)
		if err != nil {
			log.Warnf("error starting service: %v", err)
		}

	case ActionReloadUnits:
		log.Info("reloading units")
		err = m.Services.Reload(ctx)
		if err != nil {
			log.Warnf("error reloading units")
		}

	default:
		return errors.New("invalid service command")
	}
	return nil
}

func (m *Materia) Plan(ctx context.Context, man *MateriaManifest, f *Facts) ([]Action, error) {
	var actions []Action
	var err error
	if err := man.Validate(); err != nil {
		return actions, err
	}

	// Determine assigned components
	// Determine existing components
	var components map[string]*Component
	var newComponents map[string]*Component
	if components, newComponents, err = m.newDetermineComponents(ctx, man, f); err != nil {
		return actions, fmt.Errorf("error determining components: %w", err)
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
		case StateMayNeedUpdate:
			updating = append(updating, v.Name)
			log.Debug("may update:", "component", v.Name)
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
	// Determine diff actions
	diffActions, err := m.calculateDiffs(ctx, f, m.sm, components, newComponents)
	if err != nil {
		return actions, fmt.Errorf("error calculating diffs: %w", err)
	}

	// determine volume actions
	volResourceActions, err := m.calculateVolDiffs(ctx, m.sm, components)
	if err != nil {
		return actions, fmt.Errorf("error calculating volume diffs: %w", err)
	}

	// Determine response actions
	var serviceActions []Action
	// guestimate potentials
	potentialNewServices := make(map[string][]Resource)
	potentialRemovedServices := make(map[string][]Resource)
	for _, v := range diffActions {
		if v.Todo == ActionInstallResource || v.Todo == ActionUpdateResource {
			if v.Payload.Kind == ResourceTypeContainer || v.Payload.Kind == ResourceTypePod {
				potentialNewServices[v.Parent.Name] = append(potentialNewServices[v.Parent.Name], v.Payload)
			}
		}
		if v.Todo == ActionRemoveResource {
			if v.Payload.Kind == ResourceTypeContainer || v.Payload.Kind == ResourceTypePod {
				potentialRemovedServices[v.Parent.Name] = []Resource{}
			}
		}
		if v.Todo == ActionRemoveComponent {
			potentialRemovedServices[v.Parent.Name] = v.Parent.Resources
		}
	}
	for _, k := range keys {
		c := components[k]
		if c.State == StateOK {
			// TODO handle restarting service when file changes
			servs := getServicesFromResources(c.Resources)
			for _, s := range servs {
				unit, err := m.Services.Get(ctx, s.Name)
				if err != nil {
					return actions, err
				}
				if unit.State != "active" {
					serviceActions = append(serviceActions, Action{
						Todo:    ActionStartService,
						Parent:  c,
						Payload: s,
					})
				}
			}
		}
	}
	pots := sortedKeys(potentialNewServices)
	for _, compName := range pots {
		reslist := potentialNewServices[compName]
		comp, ok := components[compName]
		if !ok {
			return actions, fmt.Errorf("potential service for nonexistent component: %v", compName)
		}
		var servs []Resource
		if len(comp.Services) == 0 {
			servs = getServicesFromResources(reslist)
		} else {
			// we have provided services so we should use that instead of gustimating it
			servs = comp.Services
		}
		for _, s := range servs {
			unit, err := m.Services.Get(ctx, s.Name)
			if err != nil {
				return actions, err
			}
			if unit.State != "active" {
				serviceActions = append(serviceActions, Action{
					Todo:    ActionStartService,
					Parent:  comp,
					Payload: s,
				})
			}
		}
	}
	pots = sortedKeys(potentialRemovedServices)
	for _, compName := range pots {
		reslist := potentialRemovedServices[compName]
		comp, ok := components[compName]
		if !ok {
			return actions, fmt.Errorf("potential removed service for nonexistent component: %v", compName)
		}
		var servs []Resource
		if len(comp.Services) == 0 {
			servs = getServicesFromResources(reslist)
		} else {
			// we have provided services so we should use that instead of gustimating it
			servs = comp.Services
		}
		for _, s := range servs {
			unit, err := m.Services.Get(ctx, s.Name)
			if err != nil {
				return actions, err
			}
			if unit.State == "active" {
				serviceActions = append(serviceActions, Action{
					Todo:    ActionStopService,
					Parent:  comp,
					Payload: s,
				})
			}
		}
	}
	log.Debug("diff actions", "diffActions", diffActions)
	log.Debug("volume actions", "volResourceActions", volResourceActions)
	log.Debug("service actions", "serviceActions", serviceActions)
	actions = append(diffActions, volResourceActions...)
	actions = append(actions, serviceActions...)
	return actions, nil
}

func (m *Materia) Execute(ctx context.Context, f *Facts, plan []Action) error {
	// Template and install resources
	resourceChanged := false
	for _, v := range plan {
		vars := make(map[string]interface{})
		if err := v.Validate(); err != nil {
			return err
		}
		vaultVars := m.sm.Lookup(ctx, secrets.SecretFilter{
			Hostname:  f.Hostname,
			Role:      f.Role,
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
			if err := m.files.InstallResource(ctx, v.Parent, v.Payload, m.templateFunctions, vars); err != nil {
				return err
			}

			resourceChanged = true
		case ActionUpdateResource:
			if err := m.files.InstallResource(ctx, v.Parent, v.Payload, m.templateFunctions, vars); err != nil {
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
	// Ensure volumes and volume resources
	// start/stop services
	for _, v := range plan {
		switch v.Todo {
		case ActionInstallVolumeResource:
			err := m.files.InstallResource(ctx, v.Parent, v.Payload, m.templateFunctions, m.sm.Lookup(ctx, secrets.SecretFilter{
				Hostname:  f.Hostname,
				Role:      f.Role,
				Component: v.Parent.Name,
			}))
			if err != nil {
				return err
			}
		case ActionStartService, ActionStopService, ActionRestartService:
			err := m.modifyService(ctx, v)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Materia) Facts(ctx context.Context, c *Config) (*MateriaManifest, *Facts, error) {
	err := m.source.Sync(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error syncing source: %w", err)
	}
	man, err := m.files.GetManifest(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting repo manifest %w", err)
	}
	facts := &Facts{}
	if c.Hostname != "" {
		facts.Hostname = c.Hostname
	} else {
		facts.Hostname, err = os.Hostname()
		if err != nil {
			return nil, nil, fmt.Errorf("error getting hostname: %w", err)
		}
	}

	return man, facts, nil
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
