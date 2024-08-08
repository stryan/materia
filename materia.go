package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"git.saintnet.tech/stryan/materia/internal/secrets"
	"github.com/charmbracelet/log"
)

type Materia struct {
	prefix, quadletDestination, state string
	concerns                          map[string][]string
	Decans                            map[string]*Decan
}

func NewMateria(prefix, destination, state string) *Materia {
	return &Materia{
		prefix:             prefix,
		quadletDestination: destination,
		state:              state,
		concerns:           make(map[string][]string),
		Decans:             make(map[string]*Decan),
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

func (m *Materia) DetermineDecans() ([]applicationAction, error) {
	actions := []applicationAction{}
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
		actions = append(actions, applicationAction{
			Name:    decan.Name,
			Enabled: true,
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
			actions = append(actions, applicationAction{
				Name:    v.Name(),
				Enabled: false,
			})
		}
	}
	return actions, nil
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

func (m *Materia) ApplyDecan(decan string, sm secrets.SecretsManager) ([]applicationResult, error) {
	var results []applicationResult

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
	application := applicationResult{
		Decan: d.Name,
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
		if star.Quadlet {
			application.NeedReload = true
		}
		if newFile {
			application.NewServices = append(application.NewServices, d.ServiceForResource(star)...)
		} else {
			application.RestartServices = append(application.RestartServices, d.ServiceForResource(star)...)
		}

	}
	results = append(results, application)
	return results, nil
}

func (m *Materia) RemoveDecan(decan string) ([]applicationResult, error) {
	var results []applicationResult
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
	results = append(results, applicationResult{
		Decan:      d.Name,
		NeedReload: true,
		Removed:    true,
	})
	return results, nil
}
