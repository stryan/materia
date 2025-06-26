package materia

import (
	"bytes"
	"context"

	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/manifests"
)

type Repository interface {
	Install(ctx context.Context, path string, data *bytes.Buffer) error
	Remove(ctx context.Context, path string) error
	Exists(ctx context.Context, path string) (bool, error)
	Get(ctx context.Context, path string) (string, error)
	List(ctx context.Context) ([]string, error)
	Clean(ctx context.Context) error
}

type ComponentRepository interface {
	GetComponent(string) (*components.Component, error)
	GetResource(*components.Component, string) (components.Resource, error)
	GetManifest(*components.Component) (*manifests.ComponentManifest, error)
	InstallComponent(*components.Component) error
	ComponentExists(string) (bool, error)
	RemoveComponent(*components.Component) error
	UpdateComponent(*components.Component) error
	ReadResource(components.Resource) (string, error)
	InstallResource(components.Resource, *bytes.Buffer) error
	RemoveResource(components.Resource) error
	ListResources(*components.Component) ([]components.Resource, error)
	ListComponentNames() ([]string, error)
	RunCleanup(*components.Component) error
	RunSetup(*components.Component) error
	PurgeComponent(*components.Component) error
	PurgeComponentByName(string) error
	Clean() error
}
