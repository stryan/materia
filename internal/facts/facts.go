package facts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/containers"
	"git.saintnet.tech/stryan/materia/internal/manifests"
	"git.saintnet.tech/stryan/materia/internal/repository"
	"github.com/BurntSushi/toml"
)

type FactsProvider interface {
	Lookup(string) (any, error)
	Pretty() string

	GetAssignedComponents() []string
	GetInstalledComponents() []string
	GetVolumes() []*containers.Volume
	GetRoles() []string
	GetHostname() string
}

type FactsManager struct {
	Hostname            string
	Roles               []string
	AssignedComponents  []string
	Volumes             []*containers.Volume
	InstalledComponents []string
	Interfaces          map[string]NetworkInterfaces
}

func NewFacts(ctx context.Context, hostname string, man *manifests.MateriaManifest, compRepo repository.ComponentRepository, containers containers.ContainerManager) (*FactsManager, error) {
	facts := &FactsManager{}
	var err error
	if hostname != "" {
		facts.Hostname = hostname
	} else {
		facts.Hostname, err = os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("error getting hostname: %w", err)
		}
	}
	vols, err := containers.ListVolumes(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting container volumes: %w", err)
	}
	facts.Volumes = vols
	networks, err := GetInterfaceIPs()
	if err != nil {
		return nil, fmt.Errorf("error getting network interfaces: %w", err)
	}
	facts.Interfaces = networks

	if man == nil {
		// return just the host facts
		return facts, nil
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
	facts.InstalledComponents, err = compRepo.ListComponentNames()
	if err != nil {
		return nil, fmt.Errorf("error getting source components: %w", err)
	}
	return facts, nil
}

func (f *FactsManager) Lookup(arg string) (any, error) {
	input := strings.Split(arg, ".")
	switch input[0] {
	case "hostname":
		return f.Hostname, nil
	case "roles":
		return f.Roles, nil
	case "interface":
		if len(input) == 1 {
			return f.Interfaces, nil
		}
		if len(input) == 2 {
			return f.Interfaces[input[1]], nil
		}
		if len(input) == 3 {
			if input[2] == "ip4" {
				return f.Interfaces[input[1]].Ip4, nil
			}
			if input[2] == "ip6" {
				return f.Interfaces[input[1]].Ip6, nil
			}
			return nil, errors.New("invalid ip type")
		}
		if len(input) == 4 {
			index, err := strconv.Atoi(input[3])
			if err != nil {
				return nil, errors.New("invalid interface index")
			}
			if input[2] == "ip4" {
				return f.Interfaces[input[1]].Ip4[index], nil
			}
			if input[2] == "ip6" {
				return f.Interfaces[input[1]].Ip4[index], nil
			}
		}
	}
	return nil, errors.New("invalid fact lookup")
}

func (f *FactsManager) Pretty() string {
	var result string
	result += "Facts\n"
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
		result += fmt.Sprintf("%v ", v)
	}
	result += "\nNetworks: "
	for i, v := range f.Interfaces {
		result += fmt.Sprintf("\nInterface %v: %v", i, v)
	}

	return result
}

func (f *FactsManager) GetHostname() string {
	return f.Hostname
}

func (f *FactsManager) GetRoles() []string {
	return f.Roles
}

func (f *FactsManager) GetInstalledComponents() []string {
	return f.InstalledComponents
}

func (f *FactsManager) GetAssignedComponents() []string {
	return f.AssignedComponents
}

func (f *FactsManager) GetVolumes() []*containers.Volume {
	return f.Volumes
}
