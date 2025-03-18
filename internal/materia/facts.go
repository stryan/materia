package materia

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"git.saintnet.tech/stryan/materia/internal/containers"
	"git.saintnet.tech/stryan/materia/internal/repository"
	"github.com/BurntSushi/toml"
)

type Facts struct {
	Hostname            string
	Roles               []string
	AssignedComponents  []string
	Volumes             []*containers.Volume
	InstalledComponents map[string]*Component
}

func NewFacts(ctx context.Context, c *Config, man *MateriaManifest, compRepo *repository.ComponentRepository, containers containers.Containers) (*Facts, error) {
	facts := &Facts{}
	var err error
	if c.Hostname != "" {
		facts.Hostname = c.Hostname
	} else {
		facts.Hostname, err = os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("error getting hostname: %w", err)
		}
	}
	if man.RoleCommand != "" {
		roleStruct := struct{ Roles []string }{}
		cmd := exec.Command(man.RoleCommand)
		res, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		err = toml.Unmarshal(res, &roleStruct)
		if err != nil {
			return nil, err
		}
		facts.Roles = append(facts.Roles, roleStruct.Roles...)
	} else if host, ok := man.Hosts[facts.Hostname]; ok {
		if len(host.Roles) != 0 {
			facts.Roles = append(facts.Roles, host.Roles...)
		}
	}
	host, ok := man.Hosts["all"]
	if ok {
		facts.AssignedComponents = append(facts.AssignedComponents, host.Components...)
	}
	host, ok = man.Hosts[facts.Hostname]
	if ok {
		facts.AssignedComponents = append(facts.AssignedComponents, host.Components...)
	}
	for _, v := range facts.Roles {
		if len(man.Roles[v].Components) != 0 {
			facts.AssignedComponents = append(facts.AssignedComponents, man.Roles[v].Components...)
		}
	}
	vols, err := containers.ListVolumes(context.Background())
	if err != nil {
		return nil, err
	}
	facts.Volumes = vols
	facts.InstalledComponents = make(map[string]*Component)
	installPaths, err := compRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting source components: %w", err)
	}
	for _, v := range installPaths {
		comp, err := NewComponentFromHost(filepath.Base(v), compRepo)
		if err != nil {
			return nil, fmt.Errorf("error creating component from install: %w", err)
		}
		comp.State = StateStale
		facts.InstalledComponents[comp.Name] = comp
	}
	return facts, nil
}

func (f *Facts) Lookup(arg string) interface{} {
	switch arg {
	case "hostname":
		return f.Hostname
	case "roles":
		return f.Roles
	default:
		return ""
	}
}

func (f *Facts) Pretty() string {
	var result string
	result += fmt.Sprintf("Hostname: %v\n", f.Hostname)
	result += "Roles: "
	for _, r := range f.Roles {
		result += fmt.Sprintf("%v ", r)
	}
	result += "\nAssigned Components: "
	for _, v := range f.AssignedComponents {
		result += fmt.Sprintf("%v ", v)
	}
	result += "\nInstalled Components: "
	for _, v := range f.InstalledComponents {
		result += fmt.Sprintf("%v ", v.Name)
	}

	return result
}
