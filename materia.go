package main

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
	"github.com/charmbracelet/log"
	"github.com/coreos/go-systemd/v22/dbus"
)

type Materia struct {
	prefix, quadletDestination, state string
	concerns                          map[string][]string
	Components                        map[string]*Component
	newComponents                     map[string]*Component
	User                              *user.User
	Timeout                           int
}

func NewerMateria(c Config) *Materia {
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
	}

	return &Materia{
		prefix:             prefix,
		quadletDestination: destination,
		state:              state,
		concerns:           make(map[string][]string),
		Components:         make(map[string]*Component),
		newComponents:      make(map[string]*Component),
		User:               currentUser,
		Timeout:            timeout,
	}
}

func NewMateria(prefix, destination, state string, user *user.User, timeout int) *Materia {
	return &Materia{
		prefix:             prefix,
		quadletDestination: destination,
		state:              state,
		concerns:           make(map[string][]string),
		Components:         make(map[string]*Component),
		User:               user,
		Timeout:            timeout,
	}
}

func (m *Materia) SetupHost() error {
	if _, err := os.Stat(m.prefix); os.IsNotExist(err) {
		return fmt.Errorf("prefix %v does not exist, setup manually", m.prefix)
	}
	if _, err := os.Stat(m.quadletDestination); os.IsNotExist(err) {
		return fmt.Errorf("destination %v does not exist, setup manually", m.quadletDestination)
	}
	err := os.Mkdir(filepath.Join(m.prefix, "materia"), 0o755)
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

	return nil
}

func (m *Materia) NewDetermineDesiredComponents(ctx context.Context) error {
	// Get existing Components
	entries, err := os.ReadDir(m.AllComponentDataPaths())
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range entries {
		log.Debug("reading existing component", "component", v.Name())
		oldComp := &Component{
			Name:      v.Name(),
			Resources: []Resource{},
			State:     StateStale,
		}
		// load resources
		entries, err := os.ReadDir(m.ComponentDataPath(oldComp))
		if err != nil {
			return err
		}
		for _, v := range entries {
			newRes := Resource{
				Path:     filepath.Join(m.ComponentDataPath(oldComp), v.Name()),
				Name:     strings.TrimSuffix(v.Name(), ".gotmpl"),
				Kind:     FindResourceType(v.Name()),
				Template: isTemplate(v.Name()),
			}
			oldComp.Resources = append(oldComp.Resources, newRes)
		}
		// load quadlets
		entries, err = os.ReadDir(m.QuadletPath(oldComp))
		if err != nil {
			return err
		}
		for _, v := range entries {
			newRes := Resource{
				Path:     filepath.Join(m.QuadletPath(oldComp), v.Name()),
				Name:     strings.TrimSuffix(v.Name(), ".gotmpl"),
				Kind:     FindResourceType(v.Name()),
				Template: isTemplate(v.Name()),
			}
			oldComp.Resources = append(oldComp.Resources, newRes)
		}
		log.Debug("existing component", "component", oldComp)
		oldComp.State = StateStale
		m.Components[oldComp.Name] = oldComp
	}
	// figure out ones to add
	// TODO: map components to host, for now we just apply all of them
	entries, err = os.ReadDir(m.AllComponentSourcePaths())
	if err != nil {
		return err
	}
	var compPaths []string
	for _, v := range entries {
		if v.IsDir() {
			compPaths = append(compPaths, v.Name())
		}
	}
	for _, v := range compPaths {
		c := NewComponentFromSource(filepath.Join(m.AllComponentSourcePaths(), v))
		existing, ok := m.Components[c.Name]
		if !ok {
			c.State = StateFresh
			m.Components[c.Name] = c
		} else {
			m.newComponents[c.Name] = c
			existing.State = StateMayNeedUpdate
			m.Components[c.Name] = existing
		}
	}
	for _, v := range m.Components {
		if v.State == StateStale {
			// exists on disk but not in source, remove
			v.State = StateNeedRemoval
		}
	}

	return nil
}

func (m *Materia) CalculateDiffs(ctx context.Context, sm secrets.SecretsManager) ([]Action, error) {
	var actions []Action

	for _, v := range m.Components {
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
			candidate, ok := m.newComponents[v.Name]
			if !ok {
				return actions, errors.New("tried to replace component with nonexistent candidate")
			}
			resourceActions, err := v.Diff(candidate, sm)
			if err != nil {
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

func (m *Materia) StartComponent(ctx context.Context, name string) error {
	var conn *dbus.Conn
	var err error
	component, ok := m.Components[name]
	if !ok {
		return errors.New("tried to start invalid decan")
	}
	if m.User.Username != "root" {
		conn, err = dbus.NewUserConnectionContext(ctx)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		conn, err = dbus.NewSystemConnectionContext(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}
	callback := make(chan string)
	for _, unit := range component.Services {
		log.Info("starting service", "unit", unit)
		_, err := conn.StartUnitContext(ctx, unit.Name, "fail", callback)
		if err != nil {
			log.Warn(err)
		}
		select {
		case res := <-callback:
			log.Debug("started unit", "unit", unit, "result", res)
		case <-time.After(time.Duration(m.Timeout) * time.Second):
			log.Warn("timeout while starting unit", "unit", unit)
		}
	}
	return nil
}

func GetServicesFromResources(servs []Resource) []Resource {
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

func (m *Materia) ModifyService(ctx context.Context, command Action) error {
	var conn *dbus.Conn
	var err error
	res := command.Payload
	if res.Name == "" {
		return errors.New("modified empty service")
	}
	if res.Kind != ResourceTypeService {
		return errors.New("attempted to modify a non service resource")
	}
	if m.User.Username != "root" {
		conn, err = dbus.NewUserConnectionContext(ctx)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		conn, err = dbus.NewSystemConnectionContext(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}
	callback := make(chan string)
	switch command.Todo {
	case ActionStartService:
		log.Info("starting service", "unit", res.Name)
		_, err = conn.StartUnitContext(ctx, res.Name, "fail", callback)
		if err != nil {
			log.Warn(err)
		}
	case ActionStopService:
		log.Info("stopping service", "unit", res.Name)
		_, err = conn.StopUnitContext(ctx, res.Name, "fail", callback)
		if err != nil {
			log.Warn(err)
		}
	case ActionRestartService:
		log.Info("restarting service", "unit", res.Name)
		_, err = conn.RestartUnitContext(ctx, res.Name, "fail", callback)
		if err != nil {
			log.Warn(err)
		}
	default:
		return errors.New("invalid service command")
	}

	select {
	case result := <-callback:
		log.Debug("modified unit", "unit", res.Name, "result", result)
	case <-time.After(time.Duration(m.Timeout) * time.Second):
		log.Warn("timeout while starting unit", "unit", res.Name)
	}
	return nil
}

func (m *Materia) ReloadUnits(ctx context.Context) error {
	var conn *dbus.Conn
	var err error
	if m.User.Username != "root" {
		conn, err = dbus.NewUserConnectionContext(ctx)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		conn, err = dbus.NewSystemConnectionContext(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}
	err = conn.ReloadContext(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (m *Materia) State() string {
	return filepath.Join(m.state, "materia")
}

func (m *Materia) SourcePath() string {
	return filepath.Join(m.prefix, "materia", "source")
}

func (m *Materia) AllComponentSourcePaths() string {
	return filepath.Join(m.SourcePath(), "components")
}

func (m *Materia) ComponentSourcePath(decan *Component) string {
	return filepath.Join(m.AllComponentSourcePaths(), decan.Name)
}

func (m *Materia) ComponentDataPath(decan *Component) string {
	return filepath.Join(m.prefix, "materia", "components", decan.Name)
}

func (m *Materia) AllComponentDataPaths() string {
	return filepath.Join(m.prefix, "materia", "components")
}

func (m *Materia) InstallPath(comp *Component, r Resource) string {
	if r.Kind != ResourceTypeFile {
		return filepath.Join(m.quadletDestination, comp.Name)
	} else {
		return filepath.Join(m.prefix, "materia", "components", comp.Name)
	}
}

func (m *Materia) QuadletPath(comp *Component) string {
	return filepath.Join(m.quadletDestination, comp.Name)
}

func (m *Materia) InstallFile(decan, path string, data *bytes.Buffer) error {
	err := os.WriteFile(path, data.Bytes(), 0o755)
	if err != nil {
		return err
	}
	concerns := m.concerns[decan]
	concerns = append(concerns, path)
	m.concerns[decan] = concerns
	return nil
}

func (m *Materia) InstallComponent(comp *Component, sm secrets.SecretsManager) error {
	err := os.Mkdir(m.ComponentDataPath(comp), 0o755)
	if err != nil {
		return err
	}
	err = os.Mkdir(m.InstallPath(comp, Resource{}), 0o755)
	if err != nil {
		return err
	}

	log.Info("installed", "component", comp.Name)
	return nil
}

func (m *Materia) RemoveComponent(comp *Component, _ secrets.SecretsManager) error {
	for _, v := range comp.Resources {
		err := os.Remove(v.Path)
		if err != nil {
			return err
		}
		log.Info("removed", "resource", v.Name)
	}
	err := os.Remove(m.ComponentDataPath(comp))
	if err != nil {
		return err
	}
	log.Info("removed", "component", comp.Name)
	return nil
}

func (m *Materia) InstallResource(comp *Component, res Resource, sm secrets.SecretsManager) error {
	path := m.InstallPath(comp, res)
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
	err = m.InstallFile(comp.Name, fmt.Sprintf("%v/%v", path, res.Name), result)
	if err != nil {
		return err
	}

	log.Info("installed", "component", comp.Name, "resource", res.Name)
	return nil
}

func (m *Materia) RemoveResource(comp *Component, res Resource, _ secrets.SecretsManager) error {
	if strings.Contains(res.Path, m.SourcePath()) {
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
