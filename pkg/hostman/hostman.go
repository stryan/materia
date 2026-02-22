package hostman

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/internal/facts"
	"primamateria.systems/materia/internal/repository"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/services"
)

type HostmanConfig struct {
	Hostname            string
	Timeout             int
	RemotePodman        bool
	DryrunQuadlets      bool
	PodmanSecretsPrefix string

	DataDir     string
	QuadletDir  string
	ScriptsDir  string
	ServicesDir string
}

type HostManager struct {
	*containers.PodmanManager
	*services.ServiceManager
	*facts.HostFactsManager
	*repository.HostComponentRepository
	Scripts Repository
	Units   Repository
}

func NewHostManager(ctx context.Context, c *HostmanConfig) (*HostManager, error) {
	hostRepo, err := repository.NewHostComponentRepository(c.QuadletDir, filepath.Join(c.DataDir, "components"))
	if err != nil {
		return nil, fmt.Errorf("failed to create host component repo: %w", err)
	}
	factsm, err := facts.NewHostFacts(c.Hostname)
	if err != nil {
		return nil, fmt.Errorf("error generating facts: %w", err)
	}
	sm, err := services.NewServices(ctx, &services.ServicesConfig{
		Timeout:        c.Timeout,
		DryrunQuadlets: c.DryrunQuadlets,
	})
	if err != nil {
		log.Fatal(err)
	}
	cm, err := containers.NewPodmanManager(c.RemotePodman, c.PodmanSecretsPrefix)
	if err != nil {
		log.Fatal(err)
	}
	scriptRepo, err := repository.NewFileRepository(c.ScriptsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create script repo: %w", err)
	}
	serviceRepo, err := repository.NewFileRepository(c.ServicesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create service repo: %w", err)
	}

	return &HostManager{
		cm,
		sm,
		factsm,
		hostRepo,
		scriptRepo,
		serviceRepo,
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

func (h *HostManager) InstallScript(ctx context.Context, path string, data []byte) error {
	return h.Scripts.Install(ctx, path, data)
}

func (h *HostManager) InstallUnit(ctx context.Context, path string, data []byte) error {
	return h.Units.Install(ctx, path, data)
}

func (h *HostManager) RemoveScript(ctx context.Context, path string) error {
	return h.Scripts.Remove(ctx, path)
}

func (h *HostManager) RemoveUnit(ctx context.Context, path string) error {
	return h.Units.Remove(ctx, path)
}
