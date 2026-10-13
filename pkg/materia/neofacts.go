package materia

import (
	"context"
	"fmt"

	"primamateria.systems/materia/pkg/facts"
	"primamateria.systems/materia/pkg/manifests"
)

const factsNamespace = "materia"

type compLister interface {
	ListInstalledComponents() ([]string, error)
}

type MateriaCollector struct {
	hostname   string // gotta pass this in since it needs to be stable but overridden by configs
	config     *MateriaConfig
	manifest   *manifests.MateriaManifest
	hostLister compLister
}

type MateriaFacts struct {
	Hostname            string   `json:"hostname" toml:"hostname"`
	Roles               []string `json:"roles" toml:"roles"`
	AssignedComponents  []string `json:"assigned_components" toml:"assigned_components"`
	InstalledComponents []string `json:"installed_components" toml:"installed_components"`
}

func (c *MateriaCollector) Namespace() string {
	return factsNamespace
}

func (c *MateriaCollector) Collect(_ context.Context, accum *facts.Facts) (any, error) {
	res := MateriaFacts{
		Hostname: c.hostname,
	}
	roles, err := GetRoles(c.config, c.manifest, res.Hostname)
	if err != nil {
		return nil, err
	}
	res.Roles = roles
	assigned, err := GetAssignedComponents2(c.manifest, res.Hostname, roles)
	if err != nil {
		return nil, err
	}
	res.AssignedComponents = assigned
	installed, err := c.hostLister.ListInstalledComponents()
	if err != nil {
		return nil, fmt.Errorf("unable to list installed components: %w", err)
	}
	res.InstalledComponents = installed

	if res.AssignedComponents == nil {
		res.AssignedComponents = []string{}
	}

	if res.InstalledComponents == nil {
		res.InstalledComponents = []string{}
	}

	return res, nil
}

func ToMateriaFacts(f *facts.Facts) (MateriaFacts, error) {
	return facts.GetTypedFacts[MateriaFacts](f, factsNamespace)
}
