package executor

import (
	"bytes"
	"context"

	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/serviceman"
)

type Host interface {
	serviceman.ServiceManager
	containers.ContainerManager
	components.ComponentWriter
	InstallScript(context.Context, string, *bytes.Buffer) error
	RemoveScript(context.Context, string) error
	InstallUnit(context.Context, string, *bytes.Buffer) error
	RemoveUnit(context.Context, string) error
}
