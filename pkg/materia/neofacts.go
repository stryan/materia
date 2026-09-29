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
	hostname   string
	manifest   manifests.MateriaManifest
	hostLister compLister
}

type MateriaFacts struct {
	Roles               []string
	AssignedComponents  []string
	InstalledComponents []string
}

func (c *MateriaCollector) Namespace() string {
	return factsNamespace
}

func (c *MateriaCollector) Collect(_ context.Context, accum *facts.Facts) (any, error) {
	res := MateriaFacts{}
	host, ok := c.manifest.Hosts[c.hostname]
	var assigned []string
	if ok {
		assigned = append(assigned, host.Components...)
		if host.Roles != nil {
			res.Roles = host.Roles
			for _, r := range host.Roles {
				role, ok := c.manifest.Roles[r]
				if ok {
					assigned = append(assigned, role.Components...)
				}
			}
		}
	}
	res.AssignedComponents = assigned
	installed, err := c.hostLister.ListInstalledComponents()
	if err != nil {
		return nil, fmt.Errorf("unable to list installed components: %w", err)
	}
	res.InstalledComponents = installed

	return res, nil
}
