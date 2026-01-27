package materia

import (
	"bytes"
	"context"

	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

type Repository interface {
	Install(ctx context.Context, path string, data *bytes.Buffer) error
	Remove(ctx context.Context, path string) error
	Exists(ctx context.Context, path string) (bool, error)
	Get(ctx context.Context, path string) (string, error)
	List(ctx context.Context) ([]string, error)
	Clean(ctx context.Context) error
}

type ComponentReader interface {
	GetComponent(string) (*components.Component, error)
	GetResource(*components.Component, string) (components.Resource, error)
	GetManifest(*components.Component) (*manifests.ComponentManifest, error)
	ReadResource(components.Resource) (string, error)
	ListResources(*components.Component) ([]components.Resource, error)
	ComponentExists(string) (bool, error)
	ListComponentNames() ([]string, error)
	Clean() error
}

type ComponentWriter interface {
	InstallComponent(*components.Component) error
	RemoveComponent(*components.Component) error
	UpdateComponent(*components.Component) error
	InstallResource(components.Resource, *bytes.Buffer) error
	RemoveResource(components.Resource) error
	RunCleanup(*components.Component) error
	RunSetup(*components.Component) error
	PurgeComponent(*components.Component) error
	PurgeComponentByName(string) error
	Clean() error
}
