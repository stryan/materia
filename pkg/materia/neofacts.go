package materia

import (
	"context"
	"fmt"
	"strings"

	"primamateria.systems/materia/pkg/facts"
	"primamateria.systems/materia/pkg/manifests"
)

func (m *Materia) GetFacts(ctx context.Context) (string, error) {
	snap, err := facts.Collect(ctx, m.Collectors...)
	if err != nil {
		return "", fmt.Errorf("unable to get facts: %w", err)
	}
	hfacts, err := facts.ToHostFacts(snap)
	if err != nil {
		return "", err
	}
	mfacts, err := ToMateriaFacts(snap)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	result.WriteString("Facts\n")
	fmt.Fprintf(&result, "Hostname: %v\n", hfacts.Hostname)
	result.WriteString("Roles: ")
	for _, r := range mfacts.Roles {
		fmt.Fprintf(&result, "%v ", r)
	}
	assigned := mfacts.AssignedComponents
	result.WriteString("\nAssigned Components: ")
	for _, v := range assigned {
		fmt.Fprintf(&result, "%v ", v)
	}
	installed := mfacts.InstalledComponents
	result.WriteString("\nInstalled Components: ")
	for _, v := range installed {
		fmt.Fprintf(&result, "%v ", v)
	}
	result.WriteString("\nNetworks: ")
	for i, v := range hfacts.Interfaces {
		fmt.Fprintf(&result, "\nInterface %v: %v", i, v)
	}

	return result.String(), nil
}

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
