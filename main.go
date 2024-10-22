package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"

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

func main() {
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
	currentUser, err := user.Current()
	if err != nil {
		log.Fatalf(err.Error())
	}
	ctx := context.Background()
	prefix := "/var/lib"
	state := "/var/lib"
	destination := "/etc/systemd/system"
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30
	}
	if currentUser.Username != "root" {
		home := currentUser.HomeDir
		var found bool

		conf, found := os.LookupEnv("XDG_CONFIG_HOME")
		if !found {
			destination = fmt.Sprintf("%v/.config/containers/systemd/", home)
		} else {
			destination = fmt.Sprintf("%v/containers/systemd/", conf)
		}
		prefix, found = os.LookupEnv("XDG_DATA_HOME")
		if !found {
			prefix = fmt.Sprintf("%v/.local/share", home)
		}
		state, found = os.LookupEnv("XDG_DATA_STATE")
		if !found {
			state = fmt.Sprintf("%v/.local/state", home)
		}
	}
	m := NewMateria(prefix, destination, state, currentUser, timeout)
	err = m.SetupHost()
	if err != nil {
		log.Fatal(err)
	}
	prefix = fmt.Sprintf("%v/materia", prefix)
	source := git.NewGitSource(fmt.Sprintf("%v/source", prefix), c.GitRepo)
	err = source.Sync(ctx)
	if err != nil {
		log.Fatal(err)
	}
	// sm := mem.NewMemoryManager()
	// sm.Add("container_tag", "latest")
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
	actions, err := m.DetermineDecans(ctx)
	if err != nil {
		log.Fatal(err)
	}
	var results []ApplicationAction
	for _, v := range actions {
		var res []ApplicationAction
		switch v.Todo {
		case ApplicationActionInstall:
			res, err = m.ApplyDecan(v.Decan, sm)
		case ApplicationActionRemove:
			res, err = m.RemoveDecan(v.Decan)
		case ApplicationActionRestart:
			err = m.RestartDecan(ctx, v.Decan)
		case ApplicationActionStart:
			err = m.StartDecan(ctx, v.Decan)
		case ApplicationActionStop:
			err = m.StopDecan(ctx, v.Decan)
		}
		if err != nil {
			log.Fatal(err)
		}
		results = append(results, res...)
	}
	if len(results) > 0 {
		err = m.ReloadUnits(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}

	for _, v := range results {
		switch v.Todo {
		case ApplicationActionStart:
			err = m.StartDecan(ctx, v.Decan)
		case ApplicationActionStop:
			err = m.StopDecan(ctx, v.Decan)
		case ApplicationActionRestart:
			err = m.RestartDecan(ctx, v.Decan)
		default:
			log.Warn("Invalid secondary todo received", "action", v.Todo)
			err = nil
		}
		if err != nil {
			log.Fatal(err)
		}
	}
}
