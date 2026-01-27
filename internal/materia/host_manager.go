package materia

import (
	"bytes"
	"context"
)

type HostManager interface {
	ServiceManager
	ContainerManager
	ComponentReader
	ComponentWriter
	FactsProvider
	ListInstalledComponents() ([]string, error)
	InstallScript(context.Context, string, *bytes.Buffer) error
	RemoveScript(context.Context, string) error
	InstallUnit(context.Context, string, *bytes.Buffer) error
	RemoveUnit(context.Context, string) error
}
