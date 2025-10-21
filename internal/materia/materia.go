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
	"text/template"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/internal/attributes/age"
	fileattrs "primamateria.systems/materia/internal/attributes/file"
	"primamateria.systems/materia/internal/attributes/mem"
	"primamateria.systems/materia/internal/attributes/sops"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/pkg/manifests"
)

type MacroMap func(map[string]any) template.FuncMap

// TODO ugly hack, remove
var rootComponent = &components.Component{Name: "root"}

type Materia struct {
	Host           HostManager
	Source         SourceManager
	Manifest       *manifests.MateriaManifest
	Vault          AttributesEngine
	rootComponent  *components.Component
	Roles          []string
	macros         MacroMap
	snippets       map[string]*Snippet
	OutputDir      string
	onlyResources  bool
	debug          bool
	diffs          bool
	cleanup        bool
	cleanupVolumes bool
	backupVolumes  bool
	migrateVolumes bool
}

func setupVault(c *MateriaConfig) (AttributesEngine, error) {
	var attributesEngine AttributesEngine
	var err error
	// TODO replace this with attributes chaining
	if c.AgeConfig != nil {
		attributesEngine, err = age.NewAgeStore(*c.AgeConfig, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating age store: %w", err)
		}
		return attributesEngine, nil
	}
	if c.FileConfig != nil {
		attributesEngine, err = fileattrs.NewFileStore(*c.FileConfig, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating file store: %w", err)
		}
		return attributesEngine, nil
	}
	if c.SopsConfig != nil {
		attributesEngine, err = sops.NewSopsStore(*c.SopsConfig, c.SourceDir)
		if err != nil {
			return nil, fmt.Errorf("error creating sops store: %w", err)
		}
		return attributesEngine, nil
	}
	return mem.NewMemoryEngine(), nil
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
	snips := make(map[string]*Snippet)
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
	var roles []string

	roles, err = getRolesFromManifest(man, hm.GetHostname())
	if err != nil {
		return nil, fmt.Errorf("unable to load roles form manifest: %w", err)
	}

	return &Materia{
		Host:           hm,
		Source:         srcman,
		Manifest:       man,
		debug:          c.Debug,
		diffs:          c.Diffs,
		cleanup:        c.Cleanup,
		onlyResources:  c.OnlyResources,
		Vault:          attributes,
		OutputDir:      c.OutputDir,
		snippets:       snips,
		macros:         loadDefaultMacros(c, hm, snips),
		rootComponent:  rootComponent,
		cleanupVolumes: c.CleanupVolumes,
		backupVolumes:  c.BackupVolumes,
		migrateVolumes: c.MigrateVolumes,
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
	roles, err := getRolesFromManifest(m.Manifest, m.Host.GetHostname())
	if err != nil {
		return nil, fmt.Errorf("unable to load roles form manifest: %w", err)
	}
	for _, v := range roles {
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
	comp, err := m.Host.GetComponent(name)
	if err != nil {
		return err
	}

	removalPlan, err := m.plan(ctx, []string{comp.Name}, []string{})
	if err != nil {
		return err
	}
	_, err = m.Execute(ctx, removalPlan)
	return err
}

func (m *Materia) PlanComponent(ctx context.Context, name string, roles []string) (*Plan, error) {
	if roles != nil {
		m.Roles = roles
	}
	assigned := []string{}
	if name != "" {
		assigned = []string{name}
	}
	return m.plan(ctx, []string{}, assigned)
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

func (m *Materia) SavePlan(p *Plan, outputfile string) error {
	path := filepath.Join(m.OutputDir, outputfile)
	planOutput := planOutput{
		Timestamp: time.Now(),
		Plan:      p.PrettyLines(),
	}
	for _, a := range p.Steps() {
		if a.Todo == ActionUpdate {
			diffs := a.Content.([]diffmatchpatch.Diff)
			dmp := diffmatchpatch.New()
			before := dmp.DiffText1(diffs)
			after := dmp.DiffText2(diffs)
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
	defer func() { _ = file.Close() }()

	err = toml.NewEncoder(file).Encode(planOutput)
	if err != nil {
		return fmt.Errorf("failed to encode plan to TOML: %w", err)
	}
	return nil
}
