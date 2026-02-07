package components

import (
	"primamateria.systems/materia/pkg/manifests"
)

type ComponentReader interface {
	GetComponent(string) (*Component, error)
	GetResource(*Component, string) (Resource, error)
	GetManifest(*Component) (*manifests.ComponentManifest, error)
	ReadResource(Resource) (string, error)
	ListResources(*Component) ([]Resource, error)
	ComponentExists(string) (bool, error)
	ListComponentNames() ([]string, error)
	Clean() error
}

type ComponentWriter interface {
	InstallComponent(*Component) error
	RemoveComponent(*Component) error
	UpdateComponent(*Component) error
	InstallResource(Resource, []byte) error
	RemoveResource(Resource) error
	PurgeComponent(*Component) error
	PurgeComponentByName(string) error
	Clean() error
}
