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
	m := NewerMateria(c)
	log.Debug("dump", "materia", m)
	// PLAN
	// Setup host
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
	ctx := context.Background()
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
	for _, v := range m.Components {
		switch v.State {
		case StateFresh:
			log.Debug("fresh:", "component", v.Name)
		case StateMayNeedUpdate:
			log.Debug("update:", "component", v.Name)
		case StateNeedRemoval:
			log.Debug("remove:", "component", v.Name)
		case StateOK:
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
	// Determine diff actions
	diffActions, err := m.CalculateDiffs(ctx, sm)
	if err != nil {
		log.Fatal(err)
	}

	// Determine response actions
	var serviceActions []Action
	// guestimate potentials
	potentialServices := make(map[string][]Resource)
	var volumeActions []Action
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
				volumeActions = append(volumeActions, Action{
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
	for _, m := range m.Components {
		if m.State == StateOK {
			servs := GetServicesFromResources(m.Resources)
			for _, s := range servs {
				serviceActions = append(serviceActions, Action{
					Todo:    ActionStartService,
					Payload: s,
				})
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
			serviceActions = append(serviceActions, Action{
				Todo:    ActionStartService,
				Payload: s,
			})
		}
	}

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
	// Ensure volumes
	for _, v := range volumeActions {
		err := m.ModifyService(ctx, v)
		if err != nil {
			log.Fatal(err)
		}
	}

	// TODO Install volume resources

	// Start/stop services
	for _, v := range serviceActions {
		err := m.ModifyService(ctx, v)
		if err != nil {
			log.Fatal(err)
		}
	}
}
