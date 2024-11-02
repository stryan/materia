package main

import (
	"context"
	"errors"

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

	var err error
	err = envconfig.Process("MATERIA", &c)
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
	m, err := NewMateria(ctx, c)
	if err != nil {
		log.Fatal(err)
	}
	log.Debug("dump", "materia", m)

	log.Info("starting run")
	// PLAN
	plan, err := m.Plan(ctx)
	if err != nil {
		log.Fatal(err)
	}
	// EXECUTE
	err = m.Execute(ctx, plan)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("finishing run")
}
