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
	"time"

	"git.saintnet.tech/stryan/materia/internal/secrets"
	"github.com/charmbracelet/log"
	"github.com/coreos/go-systemd/v22/dbus"
)

type Materia struct {
	prefix, quadletDestination, state string
	concerns                          map[string][]string
	Decans                            map[string]*Decan
	User                              *user.User
	Timeout                           int
}

func NewMateria(prefix, destination, state string, user *user.User, timeout int) *Materia {
	return &Materia{
		prefix:             prefix,
		quadletDestination: destination,
		state:              state,
		concerns:           make(map[string][]string),
		Decans:             make(map[string]*Decan),
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
	err = os.Mkdir(filepath.Join(m.prefix, "materia", "decans"), 0o755)
	if err != nil && os.IsNotExist(err) {
		return err
	}

	return nil
}

func (m *Materia) DetermineDecans(ctx context.Context) ([]ApplicationAction, error) {
	actions := []ApplicationAction{}
	var decans []*Decan
	// figure out ones to add
	// TODO: map decans to host, for now we just apply all of them
	entries, err := os.ReadDir(m.AllDecanSourcePaths())
	if err != nil {
		return actions, err
	}
	var decanPaths []string
	for _, v := range entries {
		if v.IsDir() {
			decanPaths = append(decanPaths, v.Name())
		}
	}
	for _, v := range decanPaths {
		decan := NewDecan(filepath.Join(m.AllDecanSourcePaths(), v))
		decans = append(decans, decan)
		actions = append(actions, ApplicationAction{
			Decan: decan.Name,
			Todo:  ApplicationActionInstall,
		})
	}
	for _, v := range decans {
		m.Decans[v.Name] = v
	}
	// get decans to remove
	entries, err = os.ReadDir(m.AllDecanDataPaths())
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range entries {
		// TODO: this is slow
		_, ok := m.Decans[v.Name()]
		if !ok {
			m.Decans[v.Name()] = &Decan{
				Name:    v.Name(),
				Enabled: false,
			}
			actions = append(actions, ApplicationAction{
				Decan: v.Name(),
				Todo:  ApplicationActionRemove,
			})
		}
	}
	var conn *dbus.Conn
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
	defer conn.Close()
	for _, v := range m.Decans {
		currentServices, err := conn.ListUnitsByNamesContext(ctx, v.Services)
		if err != nil {
			return nil, err
		}
		for _, service := range currentServices {
			if service.ActiveState == "inactive" {
				actions = append(actions, ApplicationAction{
					Decan: v.Name,
					Todo:  ApplicationActionStart,
				})
			}
		}

	}

	return actions, nil
}

func (m *Materia) StartDecan(ctx context.Context, name string) error {
	var conn *dbus.Conn
	var err error
	decan, ok := m.Decans[name]
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
	for _, unit := range decan.Services {
		log.Info("starting service", "unit", unit)
		_, err := conn.StartUnitContext(ctx, unit, "fail", callback)
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

func (m *Materia) RestartDecan(ctx context.Context, name string) error {
	var conn *dbus.Conn
	var err error
	decan, ok := m.Decans[name]
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
	for _, unit := range decan.Services {
		log.Info("starting service", "unit", unit)
		_, err := conn.RestartUnitContext(ctx, unit, "fail", callback)
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

func (m *Materia) StopDecan(ctx context.Context, name string) error {
	var conn *dbus.Conn
	var err error
	decan, ok := m.Decans[name]
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
	for _, unit := range decan.Services {
		log.Info("starting service", "unit", unit)
		_, err := conn.StopUnitContext(ctx, unit, "fail", callback)
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

func (m *Materia) AllDecanSourcePaths() string {
	return filepath.Join(m.SourcePath(), "decans")
}

func (m *Materia) DecanSourcePath(decan *Decan) string {
	return filepath.Join(m.AllDecanSourcePaths(), decan.Name)
}

func (m *Materia) DecanDataPath(decan *Decan) string {
	return filepath.Join(m.prefix, "materia", "decans", decan.Name)
}

func (m *Materia) AllDecanDataPaths() string {
	return filepath.Join(m.prefix, "materia", "decans")
}

func (m *Materia) InstallPath(decan *Decan, r Resource) string {
	if r.Quadlet || r.Name == "" {
		return filepath.Join(m.quadletDestination, decan.Name)
	} else {
		return filepath.Join(m.prefix, "materia", "decans", decan.Name)
	}
}

func (m *Materia) InstallFile(decan, path, filename string, data *bytes.Buffer) error {
	err := os.WriteFile(filepath.Join(path, filename), data.Bytes(), 0o755)
	if err != nil {
		return err
	}
	concerns := m.concerns[decan]
	concerns = append(concerns, filepath.Join(path, filename))
	m.concerns[decan] = concerns
	return nil
}

func (m *Materia) ApplyDecan(decan string, sm secrets.SecretsManager) ([]ApplicationAction, error) {
	var results []ApplicationAction

	d, ok := m.Decans[decan]
	if !ok {
		return results, fmt.Errorf("tried to apply non existent decan %v", decan)
	}

	err := os.Mkdir(m.DecanDataPath(d), 0o755)
	if err != nil && os.IsNotExist(err) {
		return results, err
	}
	err = os.Mkdir(m.InstallPath(d, Resource{}), 0o755)
	if err != nil && os.IsNotExist(err) {
		return results, err
	}
	for _, star := range d.Resources {
		var result *bytes.Buffer
		data, err := os.ReadFile(star.Path)
		if err != nil {
			return results, err
		}
		if star.Template {
			result = bytes.NewBuffer([]byte{})
			log.Debug("applying template", "file", star.Name)
			tmpl, err := template.New(star.Name).Parse(string(data))
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
		finalDest := m.InstallPath(d, star)
		finalpath := filepath.Join(finalDest, star.Name)
		newFile := true
		if _, err := os.Stat(finalpath); !os.IsNotExist(err) {
			// file already exists, lets see if we're actually making changes
			// TODO: read and compare by chunks
			existingData, err := os.ReadFile(finalpath)
			if err != nil {
				return results, err
			}
			if bytes.Equal(result.Bytes(), existingData) {
				// no diff, skip file
				log.Debug("skipping existing unchanged file", "filename", star.Name, "destination", finalDest)
				continue
			} else {
				newFile = false
			}
		}
		log.Debug("writing file", "filename", star.Name, "destination", finalDest)
		err = m.InstallFile(d.Name, finalDest, star.Name, result)
		if err != nil {
			return results, err
		}
		if !newFile {
			for _, v := range d.ServiceForResource(star) {
				results = append(results, ApplicationAction{
					Service: v,
					Todo:    ApplicationActionStart,
				})
			}
		} else {
			for _, v := range d.ServiceForResource(star) {
				results = append(results, ApplicationAction{
					Service: v,
					Todo:    ApplicationActionRestart,
				})
			}
		}

	}
	return results, nil
}

func (m *Materia) RemoveDecan(decan string) ([]ApplicationAction, error) {
	var results []ApplicationAction
	d, ok := m.Decans[decan]
	if !ok {
		return results, fmt.Errorf("tried to remove non existent decan %v", decan)
	}
	if d.Enabled {
		return results, fmt.Errorf("tried to remove enabled decan %v", decan)
	}
	err := os.RemoveAll(filepath.Join(m.quadletDestination, d.Name))
	if err != nil {
		return results, err
	}
	err = os.RemoveAll(m.DecanDataPath(d))
	if err != nil {
		return results, err
	}
	results = append(results, ApplicationAction{
		Decan: decan,
		Todo:  ApplicationActionRemove,
	})
	return results, nil
}
