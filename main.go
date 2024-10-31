package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/secrets"
	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	"git.saintnet.tech/stryan/materia/internal/source/git"
	"github.com/charmbracelet/log"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	GitRepo          string `envconfig:"GIT_REPO"`
	Debug            bool   `envconfig:"DEBUG"`
	Timeout          int
	Secrets          string
	SecretsAgeIdents string `envconfig:"SECRETS_AGE_IDENTS"`
}

func (c *Config) Validate() error {
	if c.GitRepo == "" {
		return errors.New("need git repo location")
	}
	return nil
}

func main() {
	// Configure
	var c Config

	err := envconfig.Process("MATERIA", &c)
	if err != nil {
		log.Fatal(err.Error())
	}
	err = c.Validate()
	if err != nil {
		log.Fatal(err)
	}
	if c.Debug {
		log.Default().SetLevel(log.DebugLevel)
		log.Default().SetReportCaller(true)
	}
	ctx := context.Background()
	m := NewMateria(ctx, c)
	log.Debug("dump", "materia", m)

	log.Info("starting run")
	// PLAN
	// Setup host
	log.Info("setting up host")
	err = m.SetupHost()
	if err != nil {
		log.Fatal(err)
	}
	var sm secrets.SecretsManager
	if c.Secrets == "age" || c.Secrets == "" {
		sm, err = age.NewAgeStore(age.Config{
			IdentPath: c.SecretsAgeIdents,
			RepoPath:  m.SourcePath(),
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	// Ensure local cache
	log.Info("updating configured source cache")
	source := git.NewGitSource(m.SourcePath(), c.GitRepo)
	err = source.Sync(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Determine assigned components
	// Determine existing components
	if err := m.NewDetermineDesiredComponents(ctx); err != nil {
		log.Fatal(err)
	}
	log.Debug("component actions")
	var installing, removing, updating, ok []string
	for _, v := range m.Components {
		switch v.State {
		case StateFresh:
			installing = append(installing, v.Name)
			log.Debug("fresh:", "component", v.Name)
		case StateMayNeedUpdate:
			updating = append(updating, v.Name)
			log.Debug("update:", "component", v.Name)
		case StateNeedRemoval:
			removing = append(removing, v.Name)
			log.Debug("remove:", "component", v.Name)
		case StateOK:
			ok = append(ok, v.Name)
			log.Debug("ok:", "component", v.Name)
		case StateRemoved:
			log.Debug("removed:", "component", v.Name)
		case StateStale:
			log.Debug("stale:", "component", v.Name)
		case StateUnknown:
			log.Debug("unknown:", "component", v.Name)
		default:
			panic(fmt.Sprintf("unexpected main.ComponentLifecycle: %#v", v.State))
		}
	}
	log.Info("installing components", "installing", installing)
	log.Info("removing components", "removing", removing)
	log.Info("updating components", "updating", updating)
	log.Info("unchanged components", "unchanged", ok)
	// Determine diff actions
	diffActions, err := m.CalculateDiffs(ctx, sm)
	if err != nil {
		log.Fatal(err)
	}

	// determine volume actions
	volResourceActions, err := m.CalculateVolDiffs(ctx, sm)
	if err != nil {
		log.Fatal(err)
	}

	// Determine response actions
	var serviceActions []Action
	// guestimate potentials
	potentialServices := make(map[string][]Resource)
	var volumeServiceActions []Action
	for _, v := range diffActions {
		if v.Todo == ActionInstallResource || v.Todo == ActionUpdateResource {
			if v.Payload.Kind == ResourceTypeContainer || v.Payload.Kind == ResourceTypePod {
				potentialServices[v.Parent.Name] = append(potentialServices[v.Parent.Name], v.Payload)
			}
			if v.Payload.Kind == ResourceTypeVolume {
				// TODO maybe only do this if we have EnsureVolume actions, but we'll get to that
				volName, found := strings.CutSuffix(v.Payload.Name, ".volume")
				if !found {
					log.Warn("invalid volume name", "raw_name", v.Parent.Name)
				}
				volumeServiceActions = append(volumeServiceActions, Action{
					Todo:   ActionStartService,
					Parent: v.Parent,
					Payload: Resource{
						Name: fmt.Sprintf("%v-volume.service", volName),
						Kind: ResourceTypeService,
					},
				})
			}
		}
	}
	for _, c := range m.Components {
		if c.State == StateOK {
			servs := GetServicesFromResources(c.Resources)
			for _, s := range servs {
				us, err := m.SystemdConn.ListUnitsByNamesContext(ctx, []string{s.Name})
				if err != nil {
					log.Fatal(err)
				}
				if len(us) != 1 {
					log.Warn("somethings funky with service", "service", s.Name)
				}
				if us[0].ActiveState != "active" {
					serviceActions = append(serviceActions, Action{
						Todo:    ActionStartService,
						Payload: s,
					})
				}
			}
		}
	}

	for compName, servs := range potentialServices {
		comp := m.Components[compName]
		if len(comp.Services) != 0 {
			// already loaded from manifest, skip
			continue
		}
		servs := GetServicesFromResources(servs)
		for _, s := range servs {
			us, err := m.SystemdConn.ListUnitsByNamesContext(ctx, []string{s.Name})
			if err != nil {
				log.Fatal(err)
			}
			if len(us) != 1 {
				log.Warn("somethings funky with service", "service", s.Name)
			}
			if us[0].ActiveState != "active" {
				serviceActions = append(serviceActions, Action{
					Todo:    ActionStartService,
					Payload: s,
				})
			}
		}
	}

	volumeActions := append(volumeServiceActions, volResourceActions...)

	// EXECUTE
	log.Debug("diff actions", "diffActions", diffActions)
	log.Debug("volume actions", "volActions", volumeActions)
	log.Debug("service actions", "serviceActions", serviceActions)

	// Template and install resources

	for _, v := range diffActions {
		switch v.Todo {
		case ActionInstallComponent:
			if err := m.InstallComponent(v.Parent, sm); err != nil {
				log.Fatal(err)
			}
		case ActionInstallResource:
			if err := m.InstallResource(v.Parent, v.Payload, sm); err != nil {
				log.Fatal(err)
			}
		case ActionUpdateResource:
			if err := m.InstallResource(v.Parent, v.Payload, sm); err != nil {
				log.Fatal(err)
			}
		case ActionRemoveComponent:
			if err := m.RemoveComponent(v.Parent, sm); err != nil {
				log.Fatal(err)
			}
		case ActionRemoveResource:
			if err := m.RemoveResource(v.Parent, v.Payload, sm); err != nil {
				log.Fatal(err)
			}
		default:
			panic(fmt.Sprintf("unexpected main.ActionType: %#v", v.Todo))
		}
	}

	// If any resource actions were taken, daemon-reload
	if len(diffActions) > 0 {
		err := m.ReloadUnits(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}
	// Ensure volumes and volume resources
	for _, v := range volumeActions {
		switch v.Todo {
		case ActionInstallVolumeResource:
			err := m.InstallResource(v.Parent, v.Payload, sm)
			if err != nil {
				log.Fatal(err)
			}
		case ActionStartService:
			err := m.ModifyService(ctx, v)
			if err != nil {
				log.Fatal(err)
			}
		default:
			panic(fmt.Sprintf("unexpected main.ActionType: %#v", v.Todo))
		}
	}

	// Start/stop services
	for _, v := range serviceActions {
		err := m.ModifyService(ctx, v)
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Info("finishing run")
}
