package main

import (
	"context"
	"errors"
	"fmt"

	"git.saintnet.tech/stryan/materia/internal/secrets"
	"git.saintnet.tech/stryan/materia/internal/secrets/age"
	"git.saintnet.tech/stryan/materia/internal/source/git"
	"github.com/charmbracelet/log"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	GitRepo          string `envconfig:"GIT_REPO"`
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

// func main() {
// 	var c Config
//
// 	err := envconfig.Process("MATERIA", &c)
// 	if err != nil {
// 		log.Fatal(err.Error())
// 	}
// 	err = c.Validate()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	log.Default().SetLevel(log.DebugLevel)
// 	currentUser, err := user.Current()
// 	if err != nil {
// 		log.Fatal(err.Error())
// 	}
// 	ctx := context.Background()
// 	prefix := "/var/lib"
// 	state := "/var/lib"
// 	destination := "/etc/systemd/system"
// 	timeout := c.Timeout
// 	if timeout == 0 {
// 		timeout = 30
// 	}
// 	if currentUser.Username != "root" {
// 		home := currentUser.HomeDir
// 		var found bool
//
// 		conf, found := os.LookupEnv("XDG_CONFIG_HOME")
// 		if !found {
// 			destination = fmt.Sprintf("%v/.config/containers/systemd/", home)
// 		} else {
// 			destination = fmt.Sprintf("%v/containers/systemd/", conf)
// 		}
// 		prefix, found = os.LookupEnv("XDG_DATA_HOME")
// 		if !found {
// 			prefix = fmt.Sprintf("%v/.local/share", home)
// 		}
// 		state, found = os.LookupEnv("XDG_DATA_STATE")
// 		if !found {
// 			state = fmt.Sprintf("%v/.local/state", home)
// 		}
// 	}
// 	m := NewMateria(prefix, destination, state, currentUser, timeout)
// 	err = m.SetupHost()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	prefix = fmt.Sprintf("%v/materia", prefix)
// 	source := git.NewGitSource(fmt.Sprintf("%v/source", prefix), c.GitRepo)
// 	err = source.Sync(ctx)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	// sm := mem.NewMemoryManager()
// 	// sm.Add("container_tag", "latest")
// 	var sm secrets.SecretsManager
// 	if c.Secrets == "age" || c.Secrets == "" {
// 		sm, err = age.NewAgeStore(age.Config{
// 			IdentPath: c.SecretsAgeIdents,
// 			RepoPath:  m.SourcePath(),
// 		})
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 	}
// 	actions, err := m.DetermineComponents(ctx)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	var results []ApplicationAction
// 	for _, v := range actions {
// 		var res []ApplicationAction
// 		switch v.Todo {
// 		case ActionInstall:
// 			res, err = m.ApplyComponent(v.Decan, sm)
// 		case ActionRemove:
// 			res, err = m.RemoveComponent(v.Decan)
// 		case ActionRestart:
// 			err = m.RestartComponent(ctx, v.Decan)
// 		case ActionStart:
// 			err = m.StartComponent(ctx, v.Decan)
// 		case ActionStop:
// 			err = m.StopComponent(ctx, v.Decan)
// 		}
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		results = append(results, res...)
// 	}
// 	if len(results) > 0 {
// 		err = m.ReloadUnits(ctx)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 	}
//
// 	for _, v := range results {
// 		switch v.Todo {
// 		case ActionStart:
// 			err = m.StartComponent(ctx, v.Decan)
// 		case ActionStop:
// 			err = m.StopComponent(ctx, v.Decan)
// 		case ActionRestart:
// 			err = m.RestartComponent(ctx, v.Decan)
// 		default:
// 			log.Warn("Invalid secondary todo received", "action", v.Todo)
// 			err = nil
// 		}
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 	}
// }

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
	log.Default().SetLevel(log.DebugLevel)
	log.Default().SetReportCaller(true)
	m := NewerMateria(c)
	log.Debug("dump", "materia", m)
	// PLAN
	// Setup host
	err = m.SetupHost()
	if err != nil {
		log.Fatal(err)
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
	diffActions, err := m.CalculateDiffs(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Determine service actions
	// TODO

	// EXECUTE
	log.Debug("todo actions", "diffActions", diffActions)
	// Unlock vaults
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
	// Template and install resources
	for _, v := range diffActions {
		switch v.Todo {
		case ActionInstallComponent:
			if err := m.InstallComponent(v.Payload[0], sm); err != nil {
				log.Fatal(err)
			}
		case ActionInstallResource:
			if err := m.InstallResource(v.Payload[0], v.Payload[1], sm); err != nil {
				log.Fatal(err)
			}
		case ActionUpdateResource:
			if err := m.UpdateResource(v.Payload[0], v.Payload[1], sm); err != nil {
				log.Fatal(err)
			}
		case ActionRemoveComponent:
			if err := m.RemoveComponent(v.Payload[0], sm); err != nil {
				log.Fatal(err)
			}
		case ActionRemoveResource:
			if err := m.RemoveResource(v.Payload[0], v.Payload[1], sm); err != nil {
				log.Fatal(err)
			}
		default:
			panic(fmt.Sprintf("unexpected main.ActionType: %#v", v.Todo))
		}
	}
	// Ensure volumes

	// Install volume resources

	// Start/stop services
}
