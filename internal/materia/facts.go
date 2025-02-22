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
	Hostname   string
	Roles      []string
	Components []string
}

func NewFacts(ctx context.Context, c *Config, source source.Source, files *FileRepository) (*MateriaManifest, *Facts, error) {
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
		facts.Components = append(facts.Components, host.Components...)
	}
	host, ok = man.Hosts[facts.Hostname]
	if ok {
		facts.Components = append(facts.Components, host.Components...)
	}
	for _, v := range facts.Roles {
		if len(man.Roles[v].Components) != 0 {
			facts.Components = append(facts.Components, man.Roles[v].Components...)
		}
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
