package materia

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/BurntSushi/toml"
	"primamateria.systems/materia/pkg/facts"
	"primamateria.systems/materia/pkg/manifests"
)

const factsNamespace = "materia"

type compLister interface {
	ListInstalledComponents() ([]string, error)
}

type outputLister interface {
	List(context.Context, string, string) ([]string, error)
	Get(context.Context, string) ([]byte, error)
	Exists(context.Context, string) (bool, error)
}

type MateriaCollector struct {
	hostname   string // gotta pass this in since it needs to be stable but overridden by configs
	config     *MateriaConfig
	manifest   *manifests.MateriaManifest
	hostLister compLister
	OutputDir  outputLister
}

type MateriaFacts struct {
	Hostname            string   `json:"hostname" toml:"hostname"`
	Roles               []string `json:"roles" toml:"roles"`
	AssignedComponents  []string `json:"assigned_components" toml:"assigned_components"`
	InstalledComponents []string `json:"installed_components" toml:"installed_components"`
	LastRun             string   `json:"last_run" toml:"last_run"`
	LastRevision        string   `json:"last_revision" toml:"last_revision"`
	LastRunSuccesful    bool     `json:"last_run_succesful" toml:"last_run_succesful"`
}

func (c *MateriaCollector) Namespace() string {
	return factsNamespace
}

func (c *MateriaCollector) Validate() error {
	if c.config == nil {
		return errors.New("need config")
	}
	if c.manifest == nil {
		return errors.New("need manifest")
	}
	if c.hostLister == nil {
		return errors.New("need host component lister")
	}
	if c.hostname == "" && c.config.Hostname == "" {
		return errors.New("need hostname")
	}
	return nil
}

func (c *MateriaCollector) Collect(ctx context.Context, accum *facts.Facts) (any, error) {
	res := MateriaFacts{
		Hostname: c.hostname,
	}
	if c.config.Hostname != "" {
		res.Hostname = c.config.Hostname
	}
	roles, err := GetRoles(ctx, c.config, c.manifest, res.Hostname)
	if err != nil {
		return nil, err
	}
	res.Roles = roles
	assigned, err := GetAssignedComponents(c.manifest, res.Hostname, roles)
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
	if c.OutputDir != nil {
		runfiles, err := c.OutputDir.List(ctx, "", "lastrun")
		if err != nil {
			return nil, err
		}
		if len(runfiles) > 0 {
			// sort and flip so newest is first
			slices.Sort(runfiles)
			slices.Reverse(runfiles)

			lastRunFile := runfiles[0]
			exist, err := c.OutputDir.Exists(ctx, lastRunFile)
			if err != nil {
				return nil, err
			}
			if exist {
				data, err := c.OutputDir.Get(ctx, lastRunFile)
				if err != nil {
					return nil, err
				}
				var out executionOutput
				if _, err := toml.Decode(string(data), &out); err != nil {
					return nil, err
				}
				res.LastRevision = out.EndRevision
				res.LastRun = out.Timestamp.UTC().Format(time.RFC3339)
				res.LastRunSuccesful = (out.CompletedSteps == -1) || (out.CompletedSteps >= out.TotalSteps)
			}
		}
	}

	return res, nil
}

func ToMateriaFacts(f *facts.Facts) (MateriaFacts, error) {
	return facts.GetTypedFacts[MateriaFacts](f, factsNamespace)
}
