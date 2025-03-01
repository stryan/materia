package materia

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"git.saintnet.tech/stryan/materia/internal/source"
	"github.com/BurntSushi/toml"
)

type Facts struct {
	Hostname            string
	Roles               []string
	AssignedComponents  []string
	Volumes             []*Volume
	InstalledComponents map[string]*Component
}

func NewFacts(ctx context.Context, c *Config, source source.Source, files *FileRepository, containers Containers) (*MateriaManifest, *Facts, error) {
	err := source.Sync(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error syncing source: %w", err)
	}
	man, err := files.GetManifest(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting repo manifest %w", err)
	}
	if err := man.Validate(); err != nil {
		return nil, nil, err
	}
	facts := &Facts{}
	if c.Hostname != "" {
		facts.Hostname = c.Hostname
	} else {
		facts.Hostname, err = os.Hostname()
		if err != nil {
			return nil, nil, fmt.Errorf("error getting hostname: %w", err)
		}
	}
	if man.RoleCommand != "" {
		roleStruct := struct{ Roles []string }{}
		cmd := exec.Command(man.RoleCommand)
		res, err := cmd.Output()
		if err != nil {
			return nil, nil, err
		}
		err = toml.Unmarshal(res, &roleStruct)
		if err != nil {
			return nil, nil, err
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
		return nil, nil, err
	}
	facts.Volumes = vols
	facts.InstalledComponents = make(map[string]*Component)
	comps, err := files.GetAllInstalledComponents(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting installed components: %w", err)
	}
	for _, v := range comps {
		v.State = StateStale
		facts.InstalledComponents[v.Name] = v
	}
	return man, facts, nil
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
		result += fmt.Sprintf("%v", v.Name)
	}

	return result
}
