package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"git.saintnet.tech/stryan/materia/internal/secrets"
	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	"git.saintnet.tech/stryan/materia/internal/secrets/mem"
	"git.saintnet.tech/stryan/materia/internal/source"
	"git.saintnet.tech/stryan/materia/internal/source/git"
	"github.com/charmbracelet/log"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/volumes"
	"github.com/coreos/go-systemd/v22/dbus"
)

type Materia struct {
	prefix, quadletDestination, state string
	Timeout                           int
	SystemdConn                       *dbus.Conn
	PodmanConn                        context.Context
	sm                                secrets.SecretsManager
	source                            source.Source
}

func NewMateria(ctx context.Context, c *Config) (*Materia, error) {
	currentUser, err := user.Current()
	if err != nil {
		log.Fatal(err.Error())
	}
	prefix := "/var/lib"
	state := "/var/lib"
	destination := "/etc/systemd/system"
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30
	}
	var conn *dbus.Conn
	var podConn context.Context

	if currentUser.Username != "root" {
		home := currentUser.HomeDir
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
		state, found = os.LookupEnv("XDG_DATA_STATE")
		if !found {
			state = fmt.Sprintf("%v/.local/state", home)
		}
		conn, err = dbus.NewUserConnectionContext(ctx)
		if err != nil {
			return nil, err
		}

		podConn, err = bindings.NewConnection(context.Background(), fmt.Sprintf("unix:///run/user/%v/podman/podman.sock", currentUser.Uid))
		if err != nil {
			return nil, err
		}
	} else {
		conn, err = dbus.NewSystemConnectionContext(ctx)
		if err != nil {
			return nil, err
		}
		podConn, err = bindings.NewConnection(context.Background(), "unix:///run/podman/podman.sock")
		if err != nil {
			return nil, err
		}
	}
	sourcePath := filepath.Join(prefix, "materia", "source")

	source := git.NewGitSource(sourcePath, c.GitRepo)

	return &Materia{
		prefix:             prefix,
		quadletDestination: destination,
		state:              state,
		Timeout:            timeout,
		SystemdConn:        conn,
		PodmanConn:         podConn,
		source:             source,
	}, nil
}

func (m *Materia) Close() {
	m.SystemdConn.Close()
	// TODO do something with closing the podman context here
}

func (m *Materia) Prepare(ctx context.Context, man *MateriaManifest) error {
	if err := man.Validate(); err != nil {
		return err
	}
	var err error
	if _, err := os.Stat(m.prefix); os.IsNotExist(err) {
		return fmt.Errorf("prefix %v does not exist, setup manually", m.prefix)
	}
	if _, err := os.Stat(m.quadletDestination); os.IsNotExist(err) {
		return fmt.Errorf("destination %v does not exist, setup manually", m.quadletDestination)
	}
	err = os.Mkdir(filepath.Join(m.prefix, "materia"), 0o755)
	if err != nil && os.IsNotExist(err) {
		return err
	}
	err = os.Mkdir(filepath.Join(m.prefix, "materia", "source"), 0o755)
	if err != nil && os.IsNotExist(err) {
		return err
	}
	err = os.Mkdir(filepath.Join(m.prefix, "materia", "components"), 0o755)
	if err != nil && os.IsNotExist(err) {
		return err
	}
	switch man.Secrets {
	case "age":
		conf, ok := man.SecretsConfig.(age.Config)
		if !ok {
			return errors.New("tried to create an age secrets manager but config was not for age")
		}
		m.sm, err = age.NewAgeStore(age.Config{
			IdentPath: conf.IdentPath,
			RepoPath:  m.sourcePath(),
		})
		if err != nil {
			return err
		}

	case "mem":
		m.sm = mem.NewMemoryManager()
	default:
		m.sm = mem.NewMemoryManager()
	}
	// Ensure local cache
	log.Info("updating configured source cache")
	err = m.source.Sync(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (m *Materia) determineDesiredComponents(_ context.Context, man *MateriaManifest, facts *Facts) (map[string]*Component, map[string]*Component, error) {
	// Get existing Components
	currentComponents := make(map[string]*Component)
	newComponents := make(map[string]*Component)
	entries, err := os.ReadDir(m.allComponentDataPaths())
	if err != nil {
		return nil, nil, err
	}
	for _, v := range entries {
		log.Debug("reading existing component", "component", v.Name())
		oldComp := &Component{
			Name:      v.Name(),
			Resources: []Resource{},
			State:     StateStale,
		}
		// load resources
		entries, err := os.ReadDir(m.componentDataPath(oldComp))
		if err != nil {
			return nil, nil, err
		}
		for _, v := range entries {
			newRes := Resource{
				Path:     filepath.Join(m.componentDataPath(oldComp), v.Name()),
				Name:     strings.TrimSuffix(v.Name(), ".gotmpl"),
				Kind:     findResourceType(v.Name()),
				Template: isTemplate(v.Name()),
			}
			oldComp.Resources = append(oldComp.Resources, newRes)
		}
		// load quadlets
		entries, err = os.ReadDir(m.quadletPath(oldComp))
		if err != nil {
			return nil, nil, err
		}
		for _, v := range entries {
			newRes := Resource{
				Path:     filepath.Join(m.quadletPath(oldComp), v.Name()),
				Name:     strings.TrimSuffix(v.Name(), ".gotmpl"),
				Kind:     findResourceType(v.Name()),
				Template: isTemplate(v.Name()),
			}
			oldComp.Resources = append(oldComp.Resources, newRes)
		}
		log.Debug("existing component", "component", oldComp)
		oldComp.State = StateStale
		currentComponents[oldComp.Name] = oldComp
	}
	// figure out ones to add
	var whitelist []string
	// TODO figure out role assignments
	host, ok := man.Hosts[facts.Hostname]
	if ok {
		whitelist = append(whitelist, host.Components...)
	}
	entries, err = os.ReadDir(m.allComponentSourcePaths())
	if err != nil {
		return nil, nil, err
	}
	var compPaths []string
	for _, v := range entries {
		if v.IsDir() && slices.Contains(whitelist, v.Name()) {
			compPaths = append(compPaths, v.Name())
		}
	}
	for _, v := range compPaths {
		c, err := NewComponentFromSource(filepath.Join(m.allComponentSourcePaths(), v))
		if err != nil {
			return nil, nil, err
		}
		existing, ok := currentComponents[c.Name]
		if !ok {
			c.State = StateFresh
			currentComponents[c.Name] = c
		} else {
			c.State = StateCanidate
			newComponents[c.Name] = c
			existing.State = StateMayNeedUpdate
			currentComponents[c.Name] = existing
		}
	}
	for _, v := range currentComponents {
		if v.State == StateStale {
			// exists on disk but not in source, remove
			v.State = StateNeedRemoval
		}
	}

	return currentComponents, newComponents, nil
}

func (m *Materia) calculateDiffs(ctx context.Context, sm secrets.SecretsManager, currentComponents, newComponents map[string]*Component) ([]Action, error) {
	var actions []Action

	for _, v := range currentComponents {
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
			resourceActions, err := v.diff(candidate, sm)
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

func (m *Materia) calculateVolDiffs(ctx context.Context, sm secrets.SecretsManager, components map[string]*Component) ([]Action, error) {
	var actions []Action

	for _, v := range components {
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
				callback := make(chan string)
				_, err := m.SystemdConn.StartUnitContext(ctx, volName, "fail", callback)
				if err != nil {
					return actions, err
				}
				select {
				case result := <-callback:
					log.Debug("modified volume unit", "unit", volName, "result", result)
				case <-time.After(time.Duration(m.Timeout) * time.Second):
					log.Warn("timeout while starting volume unit", "unit", volName)
				}
				resp, err := volumes.Inspect(m.PodmanConn, fmt.Sprintf("systemd-%v", volName), nil)
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

	res := command.Payload
	if err := res.Validate(); err != nil {
		return err
	}

	if res.Kind != ResourceTypeService {
		return errors.New("attempted to modify a non service resource")
	}
	callback := make(chan string)
	switch command.Todo {
	case ActionStartService:
		log.Info("starting service", "unit", res.Name)
		_, err = m.SystemdConn.StartUnitContext(ctx, res.Name, "fail", callback)
		if err != nil {
			log.Warn(err)
		}
	case ActionStopService:
		log.Info("stopping service", "unit", res.Name)
		_, err = m.SystemdConn.StopUnitContext(ctx, res.Name, "fail", callback)
		if err != nil {
			log.Warn(err)
		}
	case ActionRestartService:
		log.Info("restarting service", "unit", res.Name)
		_, err = m.SystemdConn.RestartUnitContext(ctx, res.Name, "fail", callback)
		if err != nil {
			log.Warn(err)
		}
	case ActionReloadUnits:
		log.Info("restarting service", "unit", res.Name)
		err = m.SystemdConn.ReloadContext(ctx)
		if err != nil {
			log.Warn(err)
		}
	default:
		return errors.New("invalid service command")
	}
	if command.Todo != ActionReloadUnits {
		select {
		case result := <-callback:
			log.Debug("modified unit", "unit", res.Name, "result", result)
		case <-time.After(time.Duration(m.Timeout) * time.Second):
			log.Warn("timeout while starting unit", "unit", res.Name)
		}
	}
	return nil
}

//
// func (m *Materia) statePath() string {
// 	return filepath.Join(m.state, "materia")
// }

func (m *Materia) sourcePath() string {
	return filepath.Join(m.prefix, "materia", "source")
}

func (m *Materia) allComponentSourcePaths() string {
	return filepath.Join(m.sourcePath(), "components")
}

// func (m *Materia) componentSourcePath(component *Component) string {
// 	return filepath.Join(m.allComponentSourcePaths(), component.Name)
// }

func (m *Materia) componentDataPath(component *Component) string {
	return filepath.Join(m.prefix, "materia", "components", component.Name)
}

func (m *Materia) allComponentDataPaths() string {
	return filepath.Join(m.prefix, "materia", "components")
}

func (m *Materia) installPath(comp *Component, r Resource) string {
	if r.Kind != ResourceTypeFile {
		return filepath.Join(m.quadletDestination, comp.Name)
	} else {
		return filepath.Join(m.prefix, "materia", "components", comp.Name)
	}
}

func (m *Materia) quadletPath(comp *Component) string {
	return filepath.Join(m.quadletDestination, comp.Name)
}

func (m *Materia) installFile(path string, data *bytes.Buffer) error {
	err := os.WriteFile(path, data.Bytes(), 0o755)
	if err != nil {
		return err
	}
	return nil
}

func (m *Materia) installComponent(comp *Component, _ secrets.SecretsManager) error {
	if err := comp.Validate(); err != nil {
		return err
	}

	if comp.State != StateFresh && comp.State != StateOK {
		return errors.New("tried to install a stale component")
	}

	err := os.Mkdir(m.componentDataPath(comp), 0o755)
	if err != nil {
		return err
	}
	err = os.Mkdir(m.installPath(comp, Resource{}), 0o755)
	if err != nil {
		return err
	}

	log.Info("installed", "component", comp.Name)
	return nil
}

func (m *Materia) removeComponent(comp *Component, _ secrets.SecretsManager) error {
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
	err := os.Remove(m.componentDataPath(comp))
	if err != nil {
		return err
	}
	log.Info("removed", "component", comp.Name)
	return nil
}

func (m *Materia) installResource(comp *Component, res Resource, sm secrets.SecretsManager) error {
	if err := comp.Validate(); err != nil {
		return err
	}
	if err := res.Validate(); err != nil {
		return err
	}
	path := m.installPath(comp, res)
	var result *bytes.Buffer
	data, err := os.ReadFile(res.Path)
	if err != nil {
		return err
	}
	if res.Template {
		result = bytes.NewBuffer([]byte{})
		log.Debug("applying template", "file", res.Name)
		tmpl, err := template.New(res.Name).Parse(string(data))
		if err != nil {
			panic(err)
		}
		err = tmpl.Execute(result, sm.Lookup(context.Background(), secrets.SecretFilter{}))
		if err != nil {
			panic(err)
		}
	} else {
		result = bytes.NewBuffer(data)
	}
	log.Debug("writing file", "filename", res.Name, "destination", path)
	err = m.installFile(fmt.Sprintf("%v/%v", path, res.Name), result)
	if err != nil {
		return err
	}

	log.Info("installed", "component", comp.Name, "resource", res.Name)
	return nil
}

func (m *Materia) removeResource(comp *Component, res Resource, _ secrets.SecretsManager) error {
	if err := comp.Validate(); err != nil {
		return err
	}
	if err := res.Validate(); err != nil {
		return err
	}
	if strings.Contains(res.Path, m.sourcePath()) {
		return fmt.Errorf("tried to remove resource %v for component %v from source", res.Name, comp.Name)
	}

	log.Debug("removing file", "filename", res.Name, "destination", res.Path)
	err := os.Remove(res.Path)
	if err != nil {
		return err
	}
	log.Info("removed", "component", comp.Name, "resource", res.Name)
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
	if components, newComponents, err = m.determineDesiredComponents(ctx, man, f); err != nil {
		return actions, err
	}
	log.Debug("component actions")
	var installing, removing, updating, ok []string
	for _, v := range components {
		switch v.State {
		case StateFresh:
			installing = append(installing, v.Name)
			log.Debug("fresh:", "component", v.Name)
		case StateMayNeedUpdate:
			updating = append(updating, v.Name)
			log.Debug("update:", "component", v.Name)
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
	diffActions, err := m.calculateDiffs(ctx, m.sm, components, newComponents)
	if err != nil {
		return actions, err
	}

	// determine volume actions
	volResourceActions, err := m.calculateVolDiffs(ctx, m.sm, components)
	if err != nil {
		return actions, err
	}

	// Determine response actions
	var serviceActions []Action
	// guestimate potentials
	potentialServices := make(map[string][]Resource)
	var volumeServiceActions []Action
	for _, v := range diffActions {
		if v.Todo == ActionInstallResource || v.Todo == ActionUpdateResource {
			if v.Payload.Kind == ResourceTypeContainer || v.Payload.Kind == ResourceTypePod {
				potentialServices[v.Parent.Name] = append(potentialServices[v.Parent.Name], v.Payload)
			}
			if v.Payload.Kind == ResourceTypeVolume {
				// TODO maybe only do this if we have EnsureVolume actions, but we'll get to that
				volName, found := strings.CutSuffix(v.Payload.Name, ".volume")
				if !found {
					log.Warn("invalid volume name", "raw_name", v.Parent.Name)
				}
				volumeServiceActions = append(volumeServiceActions, Action{
					Todo:   ActionStartService,
					Parent: v.Parent,
					Payload: Resource{
						Name: fmt.Sprintf("%v-volume.service", volName),
						Kind: ResourceTypeService,
					},
				})
			}
		}
	}
	for _, c := range components {
		if c.State == StateOK {
			servs := getServicesFromResources(c.Resources)
			for _, s := range servs {
				us, err := m.SystemdConn.ListUnitsByNamesContext(ctx, []string{s.Name})
				if err != nil {
					return actions, err
				}
				if len(us) != 1 {
					log.Warn("somethings funky with service", "service", s.Name)
				}
				if us[0].ActiveState != "active" {
					serviceActions = append(serviceActions, Action{
						Todo:    ActionStartService,
						Payload: s,
					})
				}
			}
		}
	}

	for compName, reslist := range potentialServices {
		comp := components[compName]
		var servs []Resource
		if len(comp.Services) == 0 {
			servs = getServicesFromResources(reslist)
		} else {
			// we have provided services so we should use that instead of gustimating it
			servs = comp.Services
		}
		for _, s := range servs {
			us, err := m.SystemdConn.ListUnitsByNamesContext(ctx, []string{s.Name})
			if err != nil {
				return actions, err
			}
			if len(us) != 1 {
				log.Warn("somethings funky with service", "service", s.Name)
			}
			if us[0].ActiveState != "active" {
				serviceActions = append(serviceActions, Action{
					Todo:    ActionStartService,
					Payload: s,
				})
			}
		}
	}

	volumeActions := append(volumeServiceActions, volResourceActions...)
	log.Debug("diff actions", "diffActions", diffActions)
	log.Debug("volume actions", "volActions", volumeActions)
	log.Debug("service actions", "serviceActions", serviceActions)
	actions = append(diffActions, volumeActions...)
	actions = append(actions, serviceActions...)
	return actions, nil
}

func (m *Materia) Execute(ctx context.Context, plan []Action) error {
	// Template and install resources
	resourceChanged := false
	for _, v := range plan {
		if err := v.Validate(); err != nil {
			return err
		}

		switch v.Todo {
		case ActionInstallComponent:
			if err := m.installComponent(v.Parent, m.sm); err != nil {
				return err
			}
			resourceChanged = true
		case ActionInstallResource:
			if err := m.installResource(v.Parent, v.Payload, m.sm); err != nil {
				return err
			}

			resourceChanged = true
		case ActionUpdateResource:
			if err := m.installResource(v.Parent, v.Payload, m.sm); err != nil {
				return err
			}

			resourceChanged = true
		case ActionRemoveComponent:
			if err := m.removeComponent(v.Parent, m.sm); err != nil {
				return err
			}

			resourceChanged = true
		case ActionRemoveResource:
			if err := m.removeResource(v.Parent, v.Payload, m.sm); err != nil {
				return err
			}

			resourceChanged = true
		default:
		}
	}

	// If any resource actions were taken, daemon-reload
	if resourceChanged {
		err := m.modifyService(ctx, Action{Todo: ActionReloadUnits})
		if err != nil {
			return err
		}
	}
	// Ensure volumes and volume resources
	// start/stop services
	for _, v := range plan {
		switch v.Todo {
		case ActionInstallVolumeResource:
			err := m.installResource(v.Parent, v.Payload, m.sm)
			if err != nil {
				return err
			}
		case ActionStartService, ActionStopService, ActionRestartService:
			err := m.modifyService(ctx, v)
			if err != nil {
				return err
			}
		default:
			panic(fmt.Sprintf("unexpected main.ActionType: %#v", v.Todo))
		}
	}
	return nil
}

func (m *Materia) Clean(ctx context.Context) error {
	err := os.RemoveAll(m.sourcePath())
	if err != nil {
		return err
	}
	err = os.RemoveAll(m.prefix)
	if err != nil {
		return err
	}

	return nil
}

func (m *Materia) Facts(ctx context.Context, c *Config) (*MateriaManifest, *Facts, error) {
	err := m.source.Sync(ctx)
	if err != nil {
		return nil, nil, err
	}
	man, err := LoadMateriaManifest(fmt.Sprintf("%v/%v", m.sourcePath(), "MANIFEST.toml"))
	if err != nil {
		return nil, nil, err
	}
	facts := &Facts{}
	if c.Hostname != "" {
		facts.Hostname = c.Hostname
	} else {
		facts.Hostname, err = os.Hostname()
		if err != nil {
			return nil, nil, err
		}
	}

	return man, facts, nil
}
