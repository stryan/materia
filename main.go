package main

import (
	"context"

	"git.saintnet.tech/stryan/materia/internal/materia"
	"github.com/charmbracelet/log"
	"github.com/kelseyhightower/envconfig"
)

func main() {
	// Configure
	var c materia.Config

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
	m, err := materia.NewMateria(ctx, c)
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
