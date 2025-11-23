package hostman

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"slices"

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
	Scripts repository.FileRepository
	Units   repository.FileRepository
}

func NewHostManager(ctx context.Context, c *materia.MateriaConfig) (*HostManager, error) {
	hostRepo, err := repository.NewHostComponentRepository(c.QuadletDir, filepath.Join(c.MateriaDir, "materia", "components"))
	if err != nil {
		return nil, fmt.Errorf("failed to create host component repo: %w", err)
	}
	factsm, err := facts.NewHostFacts(c.Hostname)
	if err != nil {
		return nil, fmt.Errorf("error generating facts: %w", err)
	}
	sm, err := services.NewServices(ctx, &services.ServicesConfig{
		Timeout:        c.Timeout,
		DryrunQuadlets: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	cm, err := containers.NewPodmanManager(c.Remote, c.SecretsPrefix)
	if err != nil {
		log.Fatal(err)
	}
	scriptRepo, err := repository.NewFileRepository(c.ScriptsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create script repo: %w", err)
	}
	serviceRepo, err := repository.NewFileRepository(c.ServiceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create service repo: %w", err)
	}

	return &HostManager{
		cm,
		sm,
		factsm,
		hostRepo,
		*scriptRepo,
		*serviceRepo,
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

func (h *HostManager) ListInstalledComponents() ([]string, error) {
	installedComponents, err := h.ListComponentNames()
	if err != nil {
		return nil, fmt.Errorf("unable to list installed components: %w", err)
	}

	slices.Sort(installedComponents)
	return installedComponents, nil
}

func (h *HostManager) InstallScript(ctx context.Context, path string, data *bytes.Buffer) error {
	return h.Scripts.Install(ctx, path, data)
}

func (h *HostManager) InstallUnit(ctx context.Context, path string, data *bytes.Buffer) error {
	return h.Units.Install(ctx, path, data)
}

func (h *HostManager) RemoveScript(ctx context.Context, path string) error {
	return h.Scripts.Remove(ctx, path)
}

func (h *HostManager) RemoveUnit(ctx context.Context, path string) error {
	return h.Units.Remove(ctx, path)
}
