package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type Materia struct {
	prefix, quadletDestination string
}

func NewMateria(prefix, destination string) *Materia {
	return &Materia{
		prefix:             prefix,
		quadletDestination: destination,
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

func (m *Materia) SourcePath() string {
	return filepath.Join(m.prefix, "materia", "source")
}

func (m *Materia) InstallPath() string {
	return m.quadletDestination
}

func (m *Materia) AllDecanSourcePaths() string {
	return filepath.Join(m.SourcePath(), "decans")
}

func (m *Materia) DecanSourcePath(decan *Decan) string {
	return filepath.Join(m.AllDecanSourcePaths(), decan.Name)
}

func (m *Materia) DecanInstallPath(decan *Decan) string {
	return filepath.Join(m.prefix, "materia", "decans", decan.Name)
}
