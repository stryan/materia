package main

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/facts"
	"primamateria.systems/materia/internal/materia"
	"primamateria.systems/materia/internal/repository"
	"primamateria.systems/materia/internal/services"
)

type HostManager struct {
	*containers.PodmanManager
	*services.ServiceManager
	*facts.HostFactsManager
	*repository.HostComponentRepository
}

func NewHostManager(c *materia.MateriaConfig) (*HostManager, error) {
	hostRepo, err := repository.NewHostComponentRepository(c.QuadletDir, filepath.Join(c.MateriaDir, "materia", "components"))
	if err != nil {
		return nil, fmt.Errorf("failed to create host component repo: %w", err)
	}
	factsm, err := facts.NewHostFacts(c.Hostname)
	if err != nil {
		return nil, fmt.Errorf("error generating facts: %w", err)
	}
	sm, err := services.NewServices(&services.ServicesConfig{
		Timeout: c.Timeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	cm, err := containers.NewPodmanManager()
	if err != nil {
		log.Fatal(err)
	}
	return &HostManager{
		cm,
		sm,
		factsm,
		hostRepo,
	}, nil
}

func (h *HostManager) ValidateComponents() ([]string, error) {
	var invalidComps []string
	dcomps, err := h.ListComponentNames()
	if err != nil {
		return invalidComps, fmt.Errorf("can't get components from prefix: %w", err)
	}
	for _, name := range dcomps {
		_, err = h.GetComponent(name)
		if err != nil {
			log.Warn("component unable to be loaded", "component", name)
			invalidComps = append(invalidComps, name)
		}
	}

	return invalidComps, nil
}
