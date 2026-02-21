// Package materia contains the primary materia plan-execute functions. You probably don't want to be calling it
package materia

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/internal/attributes"
	"primamateria.systems/materia/internal/attributes/age"
	fileattrs "primamateria.systems/materia/internal/attributes/file"
	"primamateria.systems/materia/internal/attributes/mem"
	"primamateria.systems/materia/internal/attributes/sops"
	"primamateria.systems/materia/internal/macros"
	"primamateria.systems/materia/internal/services"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/executor"
	"primamateria.systems/materia/pkg/loader"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/plan"
	"primamateria.systems/materia/pkg/planner"
)

type Materia struct {
	Host           HostManager
	Source         SourceManager
	Manifest       *manifests.MateriaManifest
	plannerConfig  planner.PlannerConfig
	Executor       *executor.Executor
	Planner        *planner.Planner
	Vault          AttributesEngine
	Roles          []string
	macros         macros.MacroMap
	snippets       map[string]*macros.Snippet
	OutputDir      string
	defaultTimeout int
	appMode        bool
	debug          bool
}

func setupVault(c *MateriaConfig) (AttributesEngine, error) {
	var vaults []AttributesEngine
	if c.AgeConfig != nil {
		vault, err := age.NewAgeStore(*c.AgeConfig, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating age store: %w", err)
		}
		if c.Attributes == "age" {
			return vault, nil
		}
		vaults = append(vaults, vault)
	}
	if c.FileConfig != nil {
		vault, err := fileattrs.NewFileStore(*c.FileConfig, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating file store: %w", err)
		}

		if c.Attributes == "file" {
			return vault, nil
		}
		vaults = append(vaults, vault)
	}
	if c.SopsConfig != nil {
		vault, err := sops.NewSopsStore(*c.SopsConfig, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating sops store: %w", err)
		}
		if c.Attributes == "sops" {
			return vault, nil
		}

		vaults = append(vaults, vault)
	}
	if len(vaults) == 0 {
		log.Warn("No attributes engines configured: defaulting to in-memory")
		return mem.NewMemoryEngine(), nil
	}
	return NewMultiVaultEngine(vaults...)
}

func NewMateriaFromConfig(ctx context.Context, c *MateriaConfig, hm HostManager, sm SourceManager) (*Materia, error) {
	vault, err := setupVault(c)
	if err != nil {
		return nil, fmt.Errorf("failed to create attributes engine: %w", err)
	}

	return NewMateria(ctx, c, hm, vault, sm)
}

func NewMateria(ctx context.Context, c *MateriaConfig, hm HostManager, attributes AttributesEngine, srcman SourceManager) (*Materia, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	snips := make(map[string]*macros.Snippet)
	defaultSnippets := loadDefaultSnippets()
	for _, v := range defaultSnippets {
		snips[v.Name] = v
	}

	man, err := srcman.LoadManifest(manifests.MateriaManifestFile)
	if err != nil {
		return nil, fmt.Errorf("error loading manifest: %w", err)
	}
	if err := man.Validate(); err != nil {
		return nil, fmt.Errorf("invalid materia manifest: %w", err)
	}
	for _, v := range man.Snippets {
		s, err := configToSnippet(v)
		if err != nil {
			return nil, err
		}
		snips[s.Name] = s
	}
	roles := c.Roles
	if len(roles) == 0 {
		roles, err = getRolesFromManifest(man, hm.GetHostname())
		if err != nil {
			return nil, fmt.Errorf("unable to load roles form manifest: %w", err)
		}
	}
	pc := planner.PlannerConfig{
		BackupVolumes: true,
	}
	if c.PlannerConfig != nil {
		pc = *c.PlannerConfig
	}
	ec := executor.ExecutorConfig{}
	if c.ExecutorConfig != nil {
		ec = *c.ExecutorConfig
	}
	sc := services.ServicesConfig{}
	if c.ServicesConfig != nil {
		sc = *c.ServicesConfig
	}
	e := executor.NewExecutor(ec, hm, sc.Timeout)
	p := planner.NewPlanner(pc, hm)

	return &Materia{
		Host:           hm,
		Source:         srcman,
		Manifest:       man,
		debug:          c.Debug,
		defaultTimeout: sc.Timeout,
		Vault:          attributes,
		OutputDir:      c.OutputDir,
		appMode:        c.AppMode,
		snippets:       snips,
		macros:         loadDefaultMacros(c, hm, snips),
		plannerConfig:  pc,
		Executor:       e,
		Planner:        p,
		Roles:          roles,
	}, nil
}

func (m *Materia) GetAssignedComponents() ([]string, error) {
	var assignedComponents []string
	hostComps, ok := m.Manifest.Hosts["all"]
	if ok {
		assignedComponents = append(assignedComponents, hostComps.Components...)
	}
	hostComps, ok = m.Manifest.Hosts[m.Host.GetHostname()]
	if ok {
		assignedComponents = append(assignedComponents, hostComps.Components...)
	}
	for _, v := range m.Roles {
		if len(m.Manifest.Roles[v].Components) != 0 {
			assignedComponents = append(assignedComponents, m.Manifest.Roles[v].Components...)
		}
	}
	slices.Sort(assignedComponents)
	return assignedComponents, nil
}

func getRolesFromManifest(man *manifests.MateriaManifest, hostname string) ([]string, error) {
	var roles []string
	if man.RoleCommand != "" {
		roleStruct := struct{ Roles []string }{}
		cmd := exec.Command(man.RoleCommand)
		res, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		err = toml.Unmarshal(res, &roleStruct)
		if err != nil {
			return nil, err
		}
		roles = append(roles, roleStruct.Roles...)
	} else if host, ok := man.Hosts[hostname]; ok {
		if len(host.Roles) != 0 {
			roles = append(roles, host.Roles...)
		}
	}
	return roles, nil
}

func (m *Materia) Clean(ctx context.Context) error {
	err := m.Host.Clean()
	if err != nil {
		return err
	}
	err = m.Source.Clean()
	if err != nil {
		return err
	}
	return os.RemoveAll(m.OutputDir)
}

func (m *Materia) CleanComponent(ctx context.Context, name string) error {
	installedComps, err := m.Host.ListComponentNames()
	if err != nil {
		return err
	}
	isInstalled := slices.Contains(installedComps, name)
	if !isInstalled {
		return errors.New("component not installed")
	}
	hostPipeline := loader.NewHostComponentPipeline(m.Host, m.Host)
	hostComponent := components.NewComponent(name)
	err = hostPipeline.Load(ctx, hostComponent)
	if err != nil {
		return fmt.Errorf("can't load host component: %w", err)
	}

	removalPlan, err := m.Planner.Plan(ctx, "local", []*components.Component{hostComponent}, []*components.Component{})
	if err != nil {
		return err
	}
	_, err = m.Executor.Execute(ctx, removalPlan)
	return err
}

func (m *Materia) Plan(ctx context.Context) (*plan.Plan, error) {
	log.Debug("determining installed components")
	installedNames, err := m.Host.ListInstalledComponents()
	if err != nil {
		return nil, err
	}
	log.Debug("determining assigned components")
	assignedNames, err := m.GetAssignedComponents()
	if err != nil {
		return nil, err
	}
	hostname := m.Host.GetHostname()
	hostPipeline := loader.NewHostComponentPipeline(m.Host, m.Host)
	installedComponents := make([]*components.Component, 0, len(installedNames))
	for _, n := range installedNames {
		hostComponent := components.NewComponent(n)
		err := hostPipeline.Load(ctx, hostComponent)
		if err != nil {
			return nil, fmt.Errorf("can't load host component: %w", err)
		}
		installedComponents = append(installedComponents, hostComponent)
	}
	assignedComponents := make([]*components.Component, 0, len(assignedNames))
	for _, n := range assignedNames {
		attrs, err := m.Vault.Lookup(ctx, attributes.AttributesFilter{
			Hostname:  hostname,
			Roles:     m.Roles,
			Component: n,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to lookup attributes: %w", err)
		}
		overrides := make([]*manifests.ComponentManifest, 0)
		override, err := m.Manifest.GetComponentOverride(hostname, n)
		if err != nil && !errors.Is(err, manifests.ErrComponentNotAssignedToHost) {
			return nil, err
		}
		if override != nil {
			overrides = append(overrides, override)
		}
		extensions := make([]*manifests.ComponentManifest, 0)
		extension, err := m.Manifest.GetComponentExtension(hostname, n)
		if err != nil && !errors.Is(err, manifests.ErrComponentNotAssignedToHost) {
			return nil, err
		}
		if extension != nil {
			extensions = append(extensions, extension)
		}

		sourcePipeline := loader.NewSourceComponentPipeline(m.Source, m.macros, attrs, overrides, extensions)
		if m.appMode {
			err = sourcePipeline.AddStage(&loader.AppCompatibilityStage{})
			if err != nil {
				return nil, err
			}
		}
		sourceComponent := components.NewComponent(n)
		err = sourcePipeline.Load(ctx, sourceComponent)
		if err != nil {
			return nil, fmt.Errorf("error loading new components: %w", err)
		}
		assignedComponents = append(assignedComponents, sourceComponent)
	}

	actionPlan, err := m.Planner.Plan(ctx, hostname, installedComponents, assignedComponents)
	if err != nil {
		return nil, err
	}
	planValidator := plan.NewDefaultValidationPipeline(installedNames)
	return actionPlan, planValidator.Validate(actionPlan)
}

func (m *Materia) PlanComponent(ctx context.Context, name string, roles []string) (*plan.Plan, error) {
	hostname := m.Host.GetHostname()
	attrs, err := m.Vault.Lookup(ctx, attributes.AttributesFilter{
		Hostname:  hostname,
		Roles:     roles,
		Component: name,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to lookup attributes: %w", err)
	}
	overrides := make([]*manifests.ComponentManifest, 0)
	override, err := m.Manifest.GetComponentOverride(hostname, name)
	if err != nil && !errors.Is(err, manifests.ErrComponentNotAssignedToHost) {
		return nil, err
	}
	if override != nil {
		overrides = append(overrides, override)
	}
	extensions := make([]*manifests.ComponentManifest, 0)
	extension, err := m.Manifest.GetComponentExtension(hostname, name)
	if err != nil && !errors.Is(err, manifests.ErrComponentNotAssignedToHost) {
		return nil, err
	}
	if extension != nil {
		extensions = append(extensions, extension)
	}
	sourcePipeline := loader.NewSourceComponentPipeline(m.Source, m.macros, attrs, overrides, extensions)
	sourceComponent := components.NewComponent(name)
	err = sourcePipeline.Load(ctx, sourceComponent)
	if err != nil {
		return nil, fmt.Errorf("can't load host component: %w", err)
	}
	return m.Planner.Plan(ctx, hostname, []*components.Component{}, []*components.Component{sourceComponent})
}

func (m *Materia) ValidateComponents(ctx context.Context) ([]string, error) {
	var invalidComps []string
	dcomps, err := m.Host.ListComponentNames()
	if err != nil {
		return invalidComps, fmt.Errorf("can't get components from prefix: %w", err)
	}
	for _, name := range dcomps {
		_, err = m.Host.GetComponent(name)
		if err != nil {
			log.Warn("component unable to be loaded", "component", name)
			invalidComps = append(invalidComps, name)
		}
	}

	return invalidComps, nil
}

func (m *Materia) PurgeComponenet(ctx context.Context, name string) error {
	return m.Host.PurgeComponentByName(name)
}

type planOutput struct {
	Timestamp        time.Time `toml:"timestamp"`
	Plan             []string  `toml:"plan"`
	ChangedResources []change
}

type change struct {
	ResourceName, Before, After string
}

func (m *Materia) SavePlan(p *plan.Plan, outputfile string) error {
	path := filepath.Join(m.OutputDir, outputfile)
	planOutput := planOutput{
		Timestamp: time.Now(),
		Plan:      p.PrettyLines(),
	}
	for _, a := range p.Steps() {
		if a.Todo == actions.ActionUpdate {
			dmp := diffmatchpatch.New()
			before := dmp.DiffText1(a.DiffContent)
			after := dmp.DiffText2(a.DiffContent)
			planOutput.ChangedResources = append(planOutput.ChangedResources, change{
				ResourceName: a.Target.Path,
				Before:       before,
				After:        after,
			})
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create file %s: %w", path, err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Warn("error closing plan file: %v", err)
		}
	}()

	err = toml.NewEncoder(file).Encode(planOutput)
	if err != nil {
		return fmt.Errorf("failed to encode plan to TOML: %w", err)
	}
	return nil
}
