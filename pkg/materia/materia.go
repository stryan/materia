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

	"charm.land/log/v2"
	"github.com/BurntSushi/toml"
	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/pkg/actions"
	"primamateria.systems/materia/pkg/attributes"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/executor"
	"primamateria.systems/materia/pkg/facts"
	"primamateria.systems/materia/pkg/loader"
	"primamateria.systems/materia/pkg/lock"
	"primamateria.systems/materia/pkg/macros"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/notify"
	"primamateria.systems/materia/pkg/plan"
	"primamateria.systems/materia/pkg/planner"
	"primamateria.systems/materia/pkg/services"
)

type Materia struct {
	Hostname       string
	Host           HostManager
	Source         SourceManager
	Manifest       *manifests.MateriaManifest
	Executor       *executor.Executor
	Planner        *planner.Planner
	Notifier       *notify.Notifier
	Vault          AttributesEngine
	Collectors     []facts.Collector
	macroGetter    func(*facts.Facts) macros.MacroMap
	Lock           Locker
	Rollback       bool
	snippets       map[string]*macros.Snippet
	OutputDir      string
	defaultTimeout int
	appMode        bool
	debug          bool
}

func New(ctx context.Context, c *MateriaConfig, hm HostManager, srcman SourceManager, attributes AttributesEngine) (*Materia, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("invalid materia config: %w", err)
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
			return nil, fmt.Errorf("can't generate snippet %v from config: %w", v.Name, err)
		}
		snips[s.Name] = s
	}

	hostname, err := facts.ResolveHostname(c.Hostname)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve hostname: %w", err)
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
	if c.Rootless {
		cn, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		potentials, err := hm.ListContainers(ctx, containers.ContainerListFilter{})
		if err != nil {
			return nil, fmt.Errorf("passed rootless but unable to list materia containers: %w", err)
		}
		var materiaContainer *containers.Container
		for _, v := range potentials {
			if v.Hostname == cn {
				materiaContainer = v
				break
			}
		}
		if materiaContainer != nil {
			if dataSrc, ok := materiaContainer.BindMounts[DefaultDataDir]; ok {
				ec.MateriaDir = dataSrc.Source
			}
			if quadSrc, ok := materiaContainer.BindMounts[DefaultQuadletDir]; ok {
				ec.QuadletDir = quadSrc.Source
			}
			if scriptSrc, ok := materiaContainer.BindMounts[DefaultScriptsDir]; ok {
				ec.ScriptsDir = scriptSrc.Source
			}
			if serviceSrc, ok := materiaContainer.BindMounts[DefaultServiceDir]; ok {
				ec.ServiceDir = serviceSrc.Source
			}

		}
	}

	sc := services.ServicesConfig{}
	nc := notify.NotifyConfig{}
	if c.NotifyConfig != nil {
		nc = *c.NotifyConfig
	}
	socketpath := ""
	if c.ServicesConfig != nil {
		sc = *c.ServicesConfig
		socketpath = sc.DbusSocket

	}
	e := executor.NewExecutor(ec, hm, sc.Timeout)
	p := planner.NewPlanner(pc, hm)
	var l Locker
	switch c.Lock {
	case "dbus":
		l, err = lock.NewDbusLocker(socketpath)
		if err != nil {
			return nil, fmt.Errorf("unable to create dbus locker: %w", err)
		}

	case "file":
		l, err = lock.NewFileLock(filepath.Join(c.MateriaDir, "lockfile"))
		if err != nil {
			return nil, fmt.Errorf("unable to create file locker: %w", err)
		}
	default:
		// maybe make a dummy lock
		l = nil
	}
	n, err := notify.NewNotifier(nc)
	if err != nil {
		return nil, err
	}
	rollback := false
	if c.RollbackConfig != nil {
		rollback = c.RollbackConfig.Kind != ""
	}
	return &Materia{
		Host:           hm,
		Hostname:       hostname,
		Source:         srcman,
		Manifest:       man,
		debug:          c.Debug,
		defaultTimeout: sc.Timeout,
		Vault:          attributes,
		OutputDir:      c.OutputDir,
		appMode:        c.AppMode,
		snippets:       snips,
		macroGetter: func(f *facts.Facts) macros.MacroMap {
			return loadDefaultMacros(c, hm, f, snips)
		},
		Executor: e,
		Planner:  p,
		Notifier: n,
		Collectors: []facts.Collector{
			&facts.HostCollector{},
			&MateriaCollector{hostname, c, man, hm},
		},
		Lock:     l,
		Rollback: rollback,
	}, nil
}

func (m *Materia) lock(ctx context.Context) error {
	if m.Lock == nil {
		return nil
	}
	if err := m.Lock.LockOrWait(ctx); err != nil {
		return fmt.Errorf("unable to get materia dbus lock: %v", err)
	}
	return nil
}

func (m *Materia) unlock() {
	if m.Lock == nil {
		return
	}
	if err := m.Lock.Unlock(); err != nil {
		log.Warn("error unlocking after execute: %w", err)
	}
}

func GetRoles(c *MateriaConfig, man *manifests.MateriaManifest, hostname string) ([]string, error) {
	roles := c.Roles
	var err error
	if len(roles) == 0 {
		roles, err = getRolesFromManifest(man, hostname)
		if err != nil {
			return nil, fmt.Errorf("unable to load roles from manifest: %w", err)
		}
	}
	return roles, nil
}

func GetAssignedComponents2(mani *manifests.MateriaManifest, hostname string, roles []string) ([]string, error) {
	var assignedComponents []string
	hostComps, ok := mani.Hosts["all"]
	if ok {
		assignedComponents = append(assignedComponents, hostComps.Components...)
	}
	hostComps, ok = mani.Hosts[hostname]
	if ok {
		assignedComponents = append(assignedComponents, hostComps.Components...)
	}
	for _, v := range roles {
		if len(mani.Roles[v].Components) != 0 {
			assignedComponents = append(assignedComponents, mani.Roles[v].Components...)
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
			return nil, fmt.Errorf("unable to execute roles command: %w", err)
		}
		err = toml.Unmarshal(res, &roleStruct)
		if err != nil {
			return nil, fmt.Errorf("invalid roles manifest: %w", err)
		}
		roles = append(roles, roleStruct.Roles...)
	} else if host, ok := man.Hosts[hostname]; ok {
		if len(host.Roles) != 0 {
			roles = append(roles, host.Roles...)
		}
	}
	return roles, nil
}

func (m *Materia) Clean(ctx context.Context, force bool) error {
	if !force {
		installedComps, err := m.Host.ListComponentNames()
		if err != nil {
			return err
		}
		hostPipeline := loader.NewHostComponentPipeline(m.Host, m.Host)
		comps := make([]*components.Component, 0, len(installedComps))
		for _, name := range installedComps {
			hostComponent, err := components.NewComponent(name)
			if err != nil {
				return err
			}
			err = hostPipeline.Load(ctx, hostComponent)
			if err != nil {
				return fmt.Errorf("can't load host component %v: %w", name, err)
			}
			comps = append(comps, hostComponent)
		}

		removalPlan, err := m.Planner.Plan(ctx, comps, []*components.Component{})
		if err != nil {
			return err
		}
		_, err = m.Executor.Execute(ctx, removalPlan)
		if err != nil {
			return err
		}
	}
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
	hostComponent, err := components.NewComponent(name)
	if err != nil {
		return err
	}
	err = hostPipeline.Load(ctx, hostComponent)
	if err != nil {
		return fmt.Errorf("can't load host component %v: %w", name, err)
	}

	removalPlan, err := m.Planner.Plan(ctx, []*components.Component{hostComponent}, []*components.Component{})
	if err != nil {
		return err
	}
	_, err = m.Executor.Execute(ctx, removalPlan)
	return err
}

func (m *Materia) Plan(ctx context.Context) (*plan.Plan, error) {
	if err := m.lock(ctx); err != nil {
		return nil, fmt.Errorf("unable to get materia dbus lock: %v", err)
	}
	defer m.unlock()
	snap, err := facts.Collect(ctx, m.Collectors...)
	if err != nil {
		return nil, fmt.Errorf("unable to get facts: %w", err)
	}
	mfacts, err := ToMateriaFacts(snap)
	if err != nil {
		return nil, fmt.Errorf("unable to get materia facts: %w", err)
	}
	log.Debug("determining installed components")
	installedNames := mfacts.InstalledComponents
	log.Debug("determining assigned components")
	assignedNames := mfacts.AssignedComponents
	hostPipeline := loader.NewHostComponentPipeline(m.Host, m.Host)
	installedComponents := make([]*components.Component, 0, len(installedNames))
	for _, n := range installedNames {
		hostComponent, err := components.NewComponent(n)
		if err != nil {
			return nil, err
		}
		err = hostPipeline.Load(ctx, hostComponent)
		if err != nil {
			return nil, fmt.Errorf("can't load host component %v: %w", n, err)
		}
		installedComponents = append(installedComponents, hostComponent)
	}
	assignedComponents := make([]*components.Component, 0, len(assignedNames))
	for _, n := range assignedNames {
		sourceComponent, err := components.NewComponent(n)
		if err != nil {
			return nil, err
		}
		attrs, err := m.Vault.Lookup(ctx, attributes.AttributesFilter{
			Hostname:  mfacts.Hostname,
			Roles:     mfacts.Roles,
			Component: sourceComponent.Name,
			Instance:  sourceComponent.Instance,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to lookup attributes for %v:, %w", n, err)
		}
		overrides := make([]*manifests.ComponentManifest, 0)
		override, err := m.Manifest.GetComponentOverride(mfacts.Hostname, n)
		if err != nil && !errors.Is(err, manifests.ErrComponentNotAssignedToHost) {
			return nil, fmt.Errorf("unable to get component overrides: %w", err)
		}
		if override != nil {
			overrides = append(overrides, override)
		}
		extensions := make([]*manifests.ComponentManifest, 0)
		extension, err := m.Manifest.GetComponentExtension(mfacts.Hostname, n)
		if err != nil && !errors.Is(err, manifests.ErrComponentNotAssignedToHost) {
			return nil, fmt.Errorf("unable to get component extensions: %w", err)
		}
		if extension != nil {
			extensions = append(extensions, extension)
		}

		sourcePipeline := loader.NewSourceComponentPipeline(m.Source, m.macroGetter(snap), attrs, overrides, extensions)
		if m.appMode {
			err = sourcePipeline.AddStage(&loader.AppCompatibilityStage{})
			if err != nil {
				return nil, fmt.Errorf("unable to enable quadlet appfile compatibility mode: %w", err)
			}
		}
		err = sourcePipeline.Load(ctx, sourceComponent)
		if err != nil {
			return nil, fmt.Errorf("error loading source component %v: %w", n, err)
		}
		assignedComponents = append(assignedComponents, sourceComponent)
	}

	actionPlan, err := m.Planner.Plan(ctx, installedComponents, assignedComponents)
	if err != nil {
		return nil, fmt.Errorf("unable to generate plan: %w", err)
	}
	planValidator := plan.NewDefaultValidationPipeline(installedNames)
	return actionPlan, planValidator.Validate(actionPlan)
}

func (m *Materia) PlanComponent(ctx context.Context, name string, roles []string) (*plan.Plan, error) {
	if name == "" && len(roles) == 0 {
		return nil, errors.New("need component name or roles to plan")
	}

	snap, err := facts.Collect(ctx, m.Collectors...)
	if err != nil {
		return nil, fmt.Errorf("unable to get facts: %w", err)
	}
	mfacts, err := ToMateriaFacts(snap)
	if err != nil {
		return nil, fmt.Errorf("unable to get materia facts: %w", err)
	}
	attrs, err := m.Vault.Lookup(ctx, attributes.AttributesFilter{
		Hostname:  mfacts.Hostname,
		Roles:     roles,
		Component: name,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to lookup attributes: %w", err)
	}

	overrides := make([]*manifests.ComponentManifest, 0)
	override, err := m.Manifest.GetComponentOverride(mfacts.Hostname, name)
	if err != nil && !errors.Is(err, manifests.ErrComponentNotAssignedToHost) {
		return nil, fmt.Errorf("unable to get component overrides: %w", err)
	}
	if override != nil {
		overrides = append(overrides, override)
	}
	extensions := make([]*manifests.ComponentManifest, 0)
	extension, err := m.Manifest.GetComponentExtension(mfacts.Hostname, name)
	if err != nil && !errors.Is(err, manifests.ErrComponentNotAssignedToHost) {
		return nil, fmt.Errorf("unable to get component extensions: %w", err)
	}
	if extension != nil {
		extensions = append(extensions, extension)
	}
	sourcePipeline := loader.NewSourceComponentPipeline(m.Source, m.macroGetter(snap), attrs, overrides, extensions)
	sourceComponent, err := components.NewComponent(name)
	if err != nil {
		return nil, err
	}
	err = sourcePipeline.Load(ctx, sourceComponent)
	if err != nil {
		return nil, fmt.Errorf("can't load source component %v : %w", name, err)
	}
	return m.Planner.Plan(ctx, []*components.Component{}, []*components.Component{sourceComponent})
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

func (m *Materia) Close() error {
	if m.Lock != nil {
		if err := m.Lock.Close(); err != nil {
			log.Warn("error closing dbus lock connection: %w", err)
		}
	}

	if err := m.Host.Close(); err != nil {
		log.Warn("error closing host manager: %w", err)
	}
	return nil
}
