package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/charmbracelet/log"
	"github.com/containers/podman/v5/pkg/systemd/parser"
	"github.com/knadh/koanf/v2"
	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/internal/attributes"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/services"
	"primamateria.systems/materia/pkg/manifests"
)

type componentTree struct {
	host, source *components.Component
	Name         string
	FinalState   components.ComponentLifecycle
}

type PlannerConfig struct {
	CleanupQuadlets bool `toml:"cleanup_quadlets"`
	CleanupVolumes  bool `toml:"cleanup_volumes"`
	BackupVolumes   bool `toml:"backup_volumes"`
	MigrateVolumes  bool `toml:"migrate_volumes"`
}

func NewPlannerConfig(k *koanf.Koanf) (*PlannerConfig, error) {
	pc := &PlannerConfig{}
	pc.CleanupQuadlets = k.Bool("planner.cleanup_quadlets")
	pc.CleanupVolumes = k.Bool("planner.cleanup_volumes")
	if k.Exists("planner.backup_volumes") {
		pc.BackupVolumes = k.Bool("planner.backup_volumes")
	} else {
		pc.BackupVolumes = true
	}
	pc.MigrateVolumes = k.Bool("planner.migrate_volumes")

	return pc, nil
}

func (m *Materia) Plan(ctx context.Context) (*Plan, error) {
	log.Debug("determining installed components")
	installed, err := m.Host.ListInstalledComponents()
	if err != nil {
		return nil, err
	}
	log.Debug("determining assigned components")
	assigned, err := m.GetAssignedComponents()
	if err != nil {
		return nil, err
	}
	return m.plan(ctx, installed, assigned)
}

func (m *Materia) plan(ctx context.Context, installedComponents, assignedComponents []string) (*Plan, error) {
	log.Debug("starting plan")
	currentVolumes, err := m.Host.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}
	var vollist []string
	for _, v := range currentVolumes {
		vollist = append(vollist, v.Name)
	}

	plan := NewPlan(installedComponents, vollist)
	if len(installedComponents) == 0 && len(assignedComponents) == 0 {
		return plan, nil
	}
	log.Debug("building component graph")
	componentGraph, err := m.BuildComponentGraph(ctx, installedComponents, assignedComponents)
	if err != nil {
		return nil, err
	}

	log.Debug("calculating component differences")
	for _, currentTree := range componentGraph.List() {
		if currentTree.host == nil {
			currentTree.source.State = components.StateFresh
			actions, err := m.PlanFreshComponent(ctx, currentTree)
			if err != nil {
				return nil, err
			}
			err = plan.Append(actions)
			if err != nil {
				return nil, fmt.Errorf("error calculating component %v differences:%w", currentTree.Name, err)
			}
		} else if currentTree.source == nil {
			currentTree.host.State = components.StateNeedRemoval
			actions, err := m.PlanRemovedComponent(ctx, currentTree)
			if err != nil {
				return nil, err
			}
			err = plan.Append(actions)
			if err != nil {
				return nil, fmt.Errorf("error calculating component %v differences:%w", currentTree.Name, err)
			}
		} else {
			currentTree.host.State = components.StateMayNeedUpdate
		}
	}

	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("generated invalid plan: %w", err)
	}
	var installing, removing, updating, ok []string
	for _, v := range componentGraph.List() {
		switch v.FinalState {
		case components.StateFresh:
			installing = append(installing, v.Name)
			log.Debug("fresh:", "component", v.Name)
		case components.StateNeedUpdate:
			updating = append(updating, v.Name)
			log.Debug("updating:", "component", v.Name)
		case components.StateMayNeedUpdate:
			log.Warn("component still listed as may need update", "component", v.Name)
		case components.StateNeedRemoval:
			removing = append(removing, v.Name)
			log.Debug("remove:", "component", v.Name)
		case components.StateOK:
			ok = append(ok, v.Name)
			log.Debug("ok:", "component", v.Name)
		case components.StateRemoved:
			log.Debug("removed:", "component", v.Name)
		case components.StateStale:
			log.Debug("stale:", "component", v.Name)
		case components.StateUnknown:
			log.Debug("unknown:", "component", v.Name)
		default:
			panic(fmt.Sprintf("unexpected main.ComponentLifecycle: %#v", v.FinalState))
		}
	}

	log.Debug("installing components", "installing", installing)
	log.Debug("removing components", "removing", removing)
	log.Debug("updating components", "updating", updating)
	log.Debug("unchanged components", "unchanged", ok)

	return plan, nil
}

func (m *Materia) getOverride(c string) *manifests.ComponentManifest {
	hostname := m.Host.GetHostname()
	hostConfig, ok := m.Manifest.Hosts[hostname]
	if !ok {
		return nil
	}

	if o, ok := hostConfig.Overrides[c]; ok {
		return &o
	}
	return nil
}

func (m *Materia) BuildComponentGraph(ctx context.Context, installedComponents, assignedComponents []string) (*ComponentGraph, error) {
	componentGraph := NewComponentGraph()

	hostname := m.Host.GetHostname()
	log.Debug("loading host components")
	for _, v := range installedComponents {
		hostComponent, err := loadHostComponent(ctx, m.Host, v)
		if err != nil {
			return nil, fmt.Errorf("can't load host component: %w", err)
		}
		err = componentGraph.Add(&componentTree{
			Name: v,
			host: hostComponent,
		})
		if err != nil {
			return nil, err
		}

	}
	log.Debug("loading source components")
	for _, v := range assignedComponents {
		attrs := m.Vault.Lookup(ctx, attributes.AttributesFilter{
			Hostname:  hostname,
			Roles:     m.Roles,
			Component: v,
		})
		tree, err := componentGraph.Get(v)
		if errors.Is(err, ErrTreeNotFound) {
			tree = &componentTree{
				Name: v,
			}
		} else if err != nil {
			return nil, err
		}
		override := m.getOverride(v)
		sourceComponent, err := loadSourceComponent(ctx, m.Source, v, attrs, override, m.macros)
		if err != nil {
			return nil, fmt.Errorf("error loading new components: %w", err)
		}
		tree.source = sourceComponent
		err = componentGraph.Add(tree)
		if err != nil {
			return nil, err
		}
	}
	return componentGraph, nil
}

func (m *Materia) PlanFreshComponent(ctx context.Context, currentTree *componentTree) ([]Action, error) {
	resourceActions, err := generateFreshComponentResources(currentTree.source)
	if err != nil {
		return nil, fmt.Errorf("can't generate fresh resources for %v: %w", currentTree.Name, err)
	}
	if m.onlyResources {
		return resourceActions, nil
	}
	if len(resourceActions) > 0 {
		resourceActions = append(resourceActions, Action{
			Todo:   ActionReload,
			Parent: rootComponent,
			Target: components.Resource{Kind: components.ResourceTypeHost},
		})
	}
	serviceActions, err := processFreshOrUnchangedComponentServices(ctx, m.Host, currentTree.source)
	if err != nil {
		return nil, fmt.Errorf("can't plan fresh services for %v: %w", currentTree.Name, err)
	}
	currentTree.FinalState = components.StateFresh
	return append(resourceActions, serviceActions...), nil
}

func (m *Materia) PlanRemovedComponent(ctx context.Context, currentTree *componentTree) ([]Action, error) {
	resourceActions, err := generateRemovedComponentResources(ctx, m.Host, m.plannerConfig, currentTree.host)
	if err != nil {
		return nil, fmt.Errorf("can't generate removed resources for %v: %w", currentTree.Name, err)
	}
	if m.onlyResources {
		return resourceActions, nil
	}
	serviceActions, err := processRemovedComponentServices(ctx, m.Host, currentTree.host)
	if err != nil {
		return nil, fmt.Errorf("can't plan removed services for %v: %w", currentTree.Name, err)
	}
	resourceActions = append(resourceActions, Action{
		Todo:   ActionReload,
		Parent: rootComponent,
		Target: components.Resource{Kind: components.ResourceTypeHost},
	})
	currentTree.FinalState = components.StateNeedRemoval
	return append(resourceActions, serviceActions...), nil
}

func (m *Materia) PlanUpdatedComponent(ctx context.Context, currentTree *componentTree) ([]Action, error) {
	resourceActions, err := generateUpdatedComponentResources(ctx, m.Host, m.plannerConfig, currentTree.host, currentTree.source)
	if err != nil {
		return nil, fmt.Errorf("can't generate resources for %v: %w", currentTree.Name, err)
	}
	if currentTree.host.Version != components.DefaultComponentVersion {
		currentTree.host.Version = components.DefaultComponentVersion
		resourceActions = append(resourceActions, Action{
			Todo:   ActionUpdate,
			Parent: currentTree.host,
			Target: components.Resource{Parent: currentTree.source.Name, Kind: components.ResourceTypeComponent, Path: currentTree.source.Name},
		})
	}

	if m.onlyResources {
		return resourceActions, nil
	}
	var serviceActions []Action
	if len(resourceActions) > 0 {
		resourceActions = append(resourceActions, Action{
			Todo:   ActionReload,
			Parent: rootComponent,
			Target: components.Resource{Kind: components.ResourceTypeHost},
		})
		currentTree.host.State = components.StateNeedUpdate
		triggeredActions, err := generateComponentServiceTriggers(currentTree.source)
		if err != nil {
			return nil, fmt.Errorf("can't generate component service triggers for %v: %w", currentTree.Name, err)
		}
		serviceActions, err = processUpdatedComponentServices(ctx, m.Host, m.diffs, currentTree.host, currentTree.source, resourceActions, triggeredActions)
		if err != nil {
			return nil, fmt.Errorf("can't process updated services for component %v: %w", currentTree.Name, err)
		}
	} else {
		serviceActions, err = processFreshOrUnchangedComponentServices(ctx, m.Host, currentTree.host)
		if err != nil {
			return nil, fmt.Errorf("can't plan unchanged services for %v: %w", currentTree.Name, err)
		}
		if len(serviceActions) > 0 {
			currentTree.host.State = components.StateNeedUpdate
			currentTree.FinalState = components.StateNeedUpdate
		} else {
			currentTree.host.State = components.StateOK
			currentTree.FinalState = components.StateOK
		}
	}
	return append(resourceActions, serviceActions...), nil
}

func generateFreshComponentResources(comp *components.Component) ([]Action, error) {
	var actions []Action
	if err := comp.Validate(); err != nil {
		return actions, fmt.Errorf("invalid component %v: %w", comp.Name, err)
	}
	if comp.State != components.StateFresh {
		return actions, errors.New("expected fresh component")
	}

	actions = append(actions, Action{
		Todo:   ActionInstall,
		Parent: comp,
		Target: components.Resource{Parent: comp.Name, Kind: components.ResourceTypeComponent, Path: comp.Name},
	})

	for _, r := range comp.Resources.List() {
		content := ""
		if r.Kind != components.ResourceTypePodmanSecret {
			content = r.Content
		}
		actions = append(actions, Action{
			Todo:        ActionInstall,
			Parent:      comp,
			Target:      r,
			DiffContent: diffmatchpatch.New().DiffMain("", content, false),
		})
	}
	if comp.Scripted {
		actions = append(actions, Action{
			Todo:   ActionSetup,
			Parent: comp,
		})
	}
	return actions, nil
}

func generateUpdatedComponentResources(ctx context.Context, mgr HostManager, opts PlannerConfig, host *components.Component, source *components.Component) ([]Action, error) {
	var diffActions []Action
	if err := host.Validate(); err != nil {
		return diffActions, fmt.Errorf("self component invalid during comparison: %w", err)
	}
	if err := source.Validate(); err != nil {
		return diffActions, fmt.Errorf("other component invalid during comparison: %w", err)
	}

	toRemove := host.Resources.Difference(source.Resources)
	toInstall := source.Resources.Difference(host.Resources)
	potentialUpdates := host.Resources.Intersection(source.Resources)

	for _, v := range toRemove.List() {
		diffActions = append(diffActions, Action{
			Todo:        ActionRemove,
			Parent:      host,
			Target:      v,
			DiffContent: []diffmatchpatch.Diff{},
		})
		if opts.CleanupQuadlets {
			cleanupActions, err := generateCleanupResourceActions(ctx, mgr, opts, host, v)
			if err != nil {
				return diffActions, err
			}
			diffActions = append(diffActions, cleanupActions...)
		}
	}

	for _, v := range toInstall.List() {
		content := ""
		if v.Kind != components.ResourceTypePodmanSecret {
			content = v.Content
		}
		diffActions = append(diffActions, Action{
			Todo:        ActionInstall,
			Parent:      source,
			Target:      v,
			DiffContent: diffmatchpatch.New().DiffMain("", content, false),
		})
	}

	for _, conflictedResource := range potentialUpdates.List() {
		sourceRes, err := source.Resources.Get(conflictedResource.Path)
		if err != nil {
			return diffActions, err
		}
		hostResource, err := host.Resources.Get(conflictedResource.Path)
		if err != nil {
			return diffActions, err
		}
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(hostResource.Content, sourceRes.Content, false)
		if diffs == nil {
			continue
		}
		if len(diffs) > 1 || diffs[0].Type != diffmatchpatch.DiffEqual {
			diffActions = append(diffActions, Action{
				Todo:        ActionUpdate,
				Parent:      source,
				Target:      conflictedResource, // TODO should we use source resource here?
				DiffContent: diffs,
			})
			if conflictedResource.Kind == components.ResourceTypeVolume && opts.MigrateVolumes {
				volumeMigrationActions, err := generateVolumeMigrationActions(ctx, mgr, source, conflictedResource)
				if err != nil {
					return diffActions, err
				}
				diffActions = append(diffActions, volumeMigrationActions...)
			}
		}

	}

	return diffActions, nil
}

func generateRemovedComponentResources(ctx context.Context, mgr HostManager, opts PlannerConfig, comp *components.Component) ([]Action, error) {
	var actions []Action
	if err := comp.Validate(); err != nil {
		return actions, fmt.Errorf("invalid component %v: %w", comp.Name, err)
	}
	if comp.State != components.StateNeedRemoval {
		return actions, errors.New("expected to be removed component")
	}

	manifestResource, err := comp.Resources.Get(manifests.ComponentManifestFile)
	if err != nil {
		return actions, fmt.Errorf("can't get component manifest:%w", err)
	}
	resourceList := comp.Resources.List()
	slices.Reverse(resourceList)
	dirs := []components.Resource{}
	for _, r := range resourceList {
		if r.Path != manifests.ComponentManifestFile {
			if r.IsFile() {
				actions = append(actions, Action{
					Todo:        ActionRemove,
					Parent:      comp,
					Target:      r,
					DiffContent: diffmatchpatch.New().DiffMain(r.Content, "", false),
				})
			} else {
				dirs = append(dirs, r)
			}
		}
		if opts.CleanupQuadlets {
			cleanupActions, err := generateCleanupResourceActions(ctx, mgr, opts, comp, r)
			if err != nil {
				return actions, err
			}
			actions = append(actions, cleanupActions...)
		}

	}
	for _, d := range dirs {
		actions = append(actions, Action{
			Todo:   ActionRemove,
			Parent: comp,
			Target: d,
		})
	}
	if comp.Scripted {
		actions = append(actions, Action{
			Todo:   ActionCleanup,
			Parent: comp,
		})
	}
	actions = append(actions, Action{
		Todo:        ActionRemove,
		Parent:      comp,
		Target:      manifestResource,
		DiffContent: diffmatchpatch.New().DiffMain(manifestResource.Content, "", false),
	})
	actions = append(actions, Action{
		Todo:   ActionRemove,
		Parent: comp,
		Target: components.Resource{Parent: comp.Name, Kind: components.ResourceTypeComponent, Path: comp.Name},
	})
	return actions, nil
}

func generateCleanupResourceActions(ctx context.Context, mgr HostManager, opts PlannerConfig, parent *components.Component, res components.Resource) ([]Action, error) {
	var result []Action

	switch res.Kind {
	case components.ResourceTypeNetwork:
		networks, err := mgr.ListNetworks(ctx)
		if err != nil {
			return result, err
		}
		for _, n := range networks {
			if n.Name == res.HostObject {
				// TODO also check that containers aren't using it
				result = append(result, Action{
					Todo:   ActionCleanup,
					Parent: parent,
					Target: res,
				})
			}
		}
	case components.ResourceTypeBuild, components.ResourceTypeImage:
		images, err := mgr.ListImages(ctx)
		if err != nil {
			return result, err
		}
		for _, i := range images {
			if slices.Contains(i.Names, res.HostObject) {
				result = append(result, Action{
					Todo:   ActionCleanup,
					Parent: parent,
					Target: res,
				})
			}
		}
	case components.ResourceTypeVolume:
		volumes, err := mgr.ListVolumes(ctx)
		if err != nil {
			return result, err
		}
		if opts.CleanupVolumes {
			for _, v := range volumes {
				if v.Name == res.HostObject {
					if opts.BackupVolumes {
						result = append(result, Action{
							Todo:   ActionDump,
							Parent: parent,
							Target: res,
						})
					}
					result = append(result, Action{
						Todo:   ActionCleanup,
						Parent: parent,
						Target: res,
					})
				}
			}
		}
	}
	return result, nil
}

func generateComponentServiceTriggers(newComponent *components.Component) (map[string][]Action, error) {
	triggeredActions := make(map[string][]Action)
	for _, src := range newComponent.Services.List() {
		for _, trigger := range src.RestartedBy {
			triggerAction, err := getServiceAction(src, newComponent, ActionRestart)
			if err != nil {
				return triggeredActions, err
			}
			triggeredActions[trigger] = append(triggeredActions[trigger], triggerAction)
		}
		for _, trigger := range src.ReloadedBy {
			triggerAction, err := getServiceAction(src, newComponent, ActionReload)
			if err != nil {
				return triggeredActions, err
			}
			triggeredActions[trigger] = append(triggeredActions[trigger], triggerAction)
		}
	}

	return triggeredActions, nil
}

func generateVolumeMigrationActions(ctx context.Context, mgr HostManager, parent *components.Component, volumeRes components.Resource) ([]Action, error) {
	var diffActions []Action
	if volumeRes.Kind != components.ResourceTypeVolume {
		return diffActions, errors.New("non volume resource")
	}
	// ensure volume actually exists and that we have something to dump
	volumes, err := mgr.ListVolumes(ctx)
	if err != nil {
		return diffActions, fmt.Errorf("can't list volumes: %w", err)
	}
	if !slices.ContainsFunc(volumes, func(vol *containers.Volume) bool {
		return vol.Name == volumeRes.HostObject
	}) {
		return []Action{}, nil
	}
	// volume resource has been updated and volume migration has been enabled, add extra actions
	stoppedServiceActions := []Action{}
	for _, s := range parent.Services.List() {
		// stop all services so that we're safe to dump
		currentServ, err := getLiveService(ctx, mgr, parent, s)
		if err != nil {
			return diffActions, fmt.Errorf("can't get service %v when generating volume migration: %w", s.Service, err)
		}
		if currentServ.State != "active" {
			continue
		}
		stopAction, err := getServiceAction(s, parent, ActionStop)
		if err != nil {
			return diffActions, err
		}
		stopAction.Priority = 1
		diffActions = append(diffActions, stopAction)
		stoppedServiceActions = append(stoppedServiceActions, stopAction)
	}
	diffActions = append(diffActions, Action{
		Todo:     ActionDump,
		Parent:   parent,
		Target:   volumeRes,
		Priority: 1,
	})
	diffActions = append(diffActions, Action{
		Todo:     ActionCleanup,
		Parent:   parent,
		Target:   volumeRes,
		Priority: 2,
	})
	diffActions = append(diffActions, Action{
		Todo:     ActionEnsure,
		Parent:   parent,
		Target:   volumeRes,
		Priority: 4,
	})
	diffActions = append(diffActions, Action{
		Todo:     ActionImport,
		Parent:   parent,
		Target:   volumeRes,
		Priority: 4,
	})
	for _, s := range stoppedServiceActions {
		startAction := s
		startAction.Todo = ActionStart
		startAction.Priority = 5
		diffActions = append(diffActions, startAction)
	}
	return diffActions, nil
}

func serviceActionWithMetadata(parent *components.Component, targetSrc components.Resource, s manifests.ServiceResourceConfig, a ActionType) Action {
	var metadata *ActionMetadata
	if s.Timeout != 0 {
		metadata = &ActionMetadata{
			ServiceTimeout: &s.Timeout,
		}
	}
	return Action{
		Todo:     a,
		Parent:   parent,
		Target:   targetSrc,
		Metadata: metadata,
	}
}

func generateServiceRemovalActions(comp *components.Component, osrc manifests.ServiceResourceConfig) ([]Action, error) {
	var result []Action
	res := components.Resource{
		Parent: comp.Name,
		Path:   osrc.Service,
		Kind:   components.ResourceTypeService,
	}
	if osrc.Static {
		// For now we don't need metadata on Enable/Disable actions since they should be effectively instant
		result = append(result, Action{
			Todo:   ActionDisable,
			Parent: comp,
			Target: res,
		})
	}
	stopAction, err := getServiceAction(osrc, comp, ActionStop)
	if err != nil {
		return result, err
	}
	result = append(result, stopAction)
	return result, nil
}

func generateServiceInstallActions(comp *components.Component, osrc manifests.ServiceResourceConfig, liveService *services.Service) ([]Action, error) {
	var actions []Action
	if shouldEnableService(osrc, liveService) {
		// For now we don't need metadata on Enable/Disable actions since they should be effectively instant
		res := components.Resource{
			Parent: comp.Name,
			Path:   osrc.Service,
			Kind:   components.ResourceTypeService,
		}
		actions = append(actions, Action{
			Todo:   ActionEnable,
			Parent: comp,
			Target: res,
		})
	}
	if !liveService.Started() {
		startAction, err := getServiceAction(osrc, comp, ActionStart)
		if err != nil {
			return actions, err
		}
		actions = append(actions, startAction)
	}
	return actions, nil
}

func loadSourceComponent(ctx context.Context, mgr SourceManager, name string, attrs map[string]any, override *manifests.ComponentManifest, macros MacroMap) (*components.Component, error) {
	sourceComponent, err := mgr.GetComponent(name)
	if err != nil {
		return nil, fmt.Errorf("error loading new components: %w", err)
	}

	manifest, err := mgr.GetManifest(sourceComponent)
	if err != nil {
		return nil, fmt.Errorf("can't load source component %v manifest: %w", sourceComponent.Name, err)
	}
	if override != nil {
		manifest, err = manifests.MergeComponentManifests(manifest, override)
		if err != nil {
			return nil, fmt.Errorf("can't load source component %v's overrides: %w", sourceComponent.Name, err)
		}
	}
	if err := sourceComponent.ApplyManifest(manifest); err != nil {
		return nil, fmt.Errorf("can't apply source component %v manifest: %w", sourceComponent.Name, err)
	}
	attrs = attributes.MergeAttributes(attrs, sourceComponent.Defaults)
	for _, r := range sourceComponent.Resources.List() {
		if r.Kind == components.ResourceTypePodmanSecret {
			newSecret, ok := attrs[r.Path]
			if !ok {
				newSecret = ""
			}
			newSecretString, isString := newSecret.(string)
			if !isString {
				return nil, fmt.Errorf("tried to load a non-string for secret %v", r.Path)
			}
			r.Content = newSecretString
			sourceComponent.Resources.Set(r)
			continue
		}
		bodyTemplate, err := mgr.ReadResource(r)
		if err != nil {
			return nil, fmt.Errorf("can't read source resource %v/%v: %w", sourceComponent.Name, r.Name(), err)
		}
		if r.Template {
			result := bytes.NewBuffer([]byte{})
			tmpl, err := template.New("resource").Option("missingkey=error").Funcs(macros(attrs)).Parse(bodyTemplate)
			if err != nil {
				return nil, err
			}
			err = tmpl.Execute(result, attrs)
			if err != nil {
				return nil, err
			}

			r.Content = result.String()
		} else {
			r.Content = bodyTemplate
		}
		if r.IsQuadlet() {
			hostObject, err := r.GetHostObject(r.Content)
			if err != nil {
				return nil, err
			}
			r.HostObject = hostObject
		}
		sourceComponent.Resources.Set(r)
	}
	return sourceComponent, nil
}

func loadHostComponent(ctx context.Context, mgr HostManager, name string) (*components.Component, error) {
	hostComponent, err := mgr.GetComponent(name)
	if err != nil {
		return nil, fmt.Errorf("can't load host component: %w", err)
	}
	if hostComponent.Version == components.DefaultComponentVersion {
		manifest, err := mgr.GetManifest(hostComponent)
		if err != nil {
			return nil, fmt.Errorf("can't load host component %v manifest: %w", hostComponent.Name, err)
		}
		if err := hostComponent.ApplyManifest(manifest); err != nil {
			return nil, fmt.Errorf("can't apply host component %v manifest: %w", hostComponent.Name, err)
		}
	}
	var deletedSecrets []components.Resource
	for _, r := range hostComponent.Resources.List() {
		if r.Kind == components.ResourceTypePodmanSecret {
			secretsList, err := mgr.ListSecrets(ctx)
			if err != nil {
				return nil, fmt.Errorf("error listing secrets during resource validation")
			}
			if !slices.Contains(secretsList, r.Path) {
				// secret isn't there so we treat it like the resource never existed
				deletedSecrets = append(deletedSecrets, r)
			} else {
				curSecret, err := mgr.GetSecret(ctx, r.Path)
				if err != nil {
					return nil, err
				}
				r.Content = curSecret.Value
				hostComponent.Resources.Set(r)
			}
			continue
		}
		body, err := mgr.ReadResource(r)
		if err != nil {
			return nil, fmt.Errorf("can't read host resource %v/%v: %w", hostComponent.Name, r.Name(), err)
		}
		r.Content = body
		if r.IsQuadlet() {
			hostObject, err := r.GetHostObject(body)
			if err != nil {
				return nil, err
			}
			r.HostObject = hostObject
		}
		hostComponent.Resources.Set(r)
	}
	for _, r := range deletedSecrets {
		hostComponent.Resources.Delete(r.Path)
	}
	return hostComponent, nil
}

func getLiveService(ctx context.Context, mgr HostManager, parent *components.Component, src manifests.ServiceResourceConfig) (*services.Service, error) {
	if src.Service == "" {
		return nil, errors.New("tried to get empty live service")
	}
	name := ""
	res, err := parent.Resources.Get(src.Service)
	if err != nil {
		name = src.Service
	} else {
		name = res.Service()
	}
	liveService, err := mgr.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return liveService, nil
}

func processFreshOrUnchangedComponentServices(ctx context.Context, mgr HostManager, component *components.Component) ([]Action, error) {
	var actions []Action
	if component.Services == nil {
		return actions, nil
	}

	for _, s := range component.Services.List() {
		if s.Stopped {
			continue
		}
		liveService, err := getLiveService(ctx, mgr, component, s)
		if err != nil {
			return actions, fmt.Errorf("can't get live service for %v: %w", s.Service, err)
		}

		installActions, err := generateServiceInstallActions(component, s, liveService)
		if err != nil {
			return actions, fmt.Errorf("can't generate install actions for %v: %w", s.Service, err)
		}
		actions = append(actions, installActions...)
	}

	return actions, nil
}

func processRemovedComponentServices(ctx context.Context, mgr HostManager, comp *components.Component) ([]Action, error) {
	var actions []Action
	if comp.Services == nil {
		return actions, nil
	}
	for _, s := range comp.Services.List() {
		liveService, err := getLiveService(ctx, mgr, comp, s)
		if err != nil {
			return actions, fmt.Errorf("can't get live service for %v: %w", s.Service, err)
		}
		if liveService.Started() {
			stopAction, err := getServiceAction(s, comp, ActionStop)
			if err != nil {
				return actions, fmt.Errorf("can't generate removal actions for %v: %w", s.Service, err)
			}
			actions = append(actions, stopAction)
		}
	}
	return actions, nil
}

func processUpdatedComponentServices(ctx context.Context, host HostManager, showDiffs bool, original, newComponent *components.Component, resourceActions []Action, triggeredActions map[string][]Action) ([]Action, error) {
	var actions []Action
	var triggeredServices []string

	for _, d := range resourceActions {
		if updatedServiceActions, ok := triggeredActions[d.Target.Path]; ok {
			actions = append(actions, updatedServiceActions...)
			for _, a := range updatedServiceActions {
				triggeredServices = append(triggeredServices, a.Target.Path)
			}
		} else if (d.Target.Kind == components.ResourceTypeContainer || d.Target.Kind == components.ResourceTypePod) && d.Todo == ActionUpdate && !newComponent.Settings.NoRestart {
			restartAction, err := resourceActionWithMetadata(d.Target, newComponent, ActionRestart)
			if err != nil {
				return actions, fmt.Errorf("error generating auto-restart option for resource %v: %w", d.Target.Path, err)
			}
			actions = append(actions, restartAction)
		}
	}

	for _, k := range newComponent.Services.List() {
		if k.Stopped {
			continue
		}
		if slices.Contains(triggeredServices, k.Service) {
			continue
		}

		liveService, err := getLiveService(ctx, host, newComponent, k)
		if err != nil {
			return nil, fmt.Errorf("can't get live service for %v:%w", k.Service, err)
		}

		installActions, err := generateServiceInstallActions(newComponent, k, liveService)
		if err != nil {
			return nil, fmt.Errorf("can't generate install actions for %v: %w", k.Service, err)
		}
		actions = append(actions, installActions...)
	}

	for _, osrc := range original.Services.List() {
		if !slices.Contains(newComponent.Services.ListServiceNames(), osrc.Service) {
			removalActions, err := generateServiceRemovalActions(original, osrc)
			if err != nil {
				return nil, fmt.Errorf("can't generate removal actions for %v:%w", osrc.Service, err)
			}
			actions = append(actions, removalActions...)
		}
	}

	return actions, nil
}

func shouldEnableService(s manifests.ServiceResourceConfig, liveService *services.Service) bool {
	return !s.Disabled && s.Static && !liveService.Enabled
}

func getServiceAction(src manifests.ServiceResourceConfig, parent *components.Component, a ActionType) (Action, error) {
	res, err := parent.Resources.Get(src.Service)
	if err != nil {
		// No resource for component, treat it like an arbitary systemd unit
		res = components.Resource{
			Parent: parent.Name,
			Path:   src.Service,
			Kind:   components.ResourceTypeService,
		}
		return serviceActionWithMetadata(parent, res, src, a), nil
	}
	return resourceActionWithMetadata(res, parent, a)
}

func resourceActionWithMetadata(res components.Resource, parent *components.Component, a ActionType) (Action, error) {
	if !a.IsServiceAction() {
		return Action{}, fmt.Errorf("can't create resource action with metadata from type %v", a)
	}

	if !res.IsQuadlet() && res.Kind != components.ResourceTypeService {
		return Action{}, fmt.Errorf("can't create resource service action from non-quadlet %v", res)
	}

	if res.Kind != components.ResourceTypeContainer && res.Kind != components.ResourceTypePod {
		// TODO volumes can be based off images too and need their timeouts adjusted accordingly
		return Action{
			Todo:   a,
			Parent: parent,
			Target: res,
		}, nil
	}

	if res.Kind == components.ResourceTypePod {
		// TODO figure out how to calculate timeout for pod
		timeout := 60000 // 10 minutes
		return Action{
			Todo:   a,
			Parent: parent,
			Target: res,
			Metadata: &ActionMetadata{
				ServiceTimeout: &timeout,
			},
		}, nil
	}
	unitfile := parser.NewUnitFile()
	err := unitfile.Parse(res.Content)
	if err != nil {
		return Action{}, fmt.Errorf("error parsing systemd unit file: %w", err)
	}
	imageName, ok := unitfile.Lookup("Container", "Image")
	if !ok {
		return Action{}, fmt.Errorf("invalid container quadlet: %v", res)
	}
	if strings.HasSuffix(imageName, ".image") || strings.HasSuffix(imageName, ".build") {
		timeout := 60
		src, err := parent.Services.Get(imageName)
		if errors.Is(err, components.ErrServiceNotFound) {
			// no custom timeout defined
			return Action{
				Todo:   a,
				Parent: parent,
				Target: res,
				Metadata: &ActionMetadata{
					ServiceTimeout: &timeout,
				},
			}, nil
		} else if err != nil {
			return Action{}, fmt.Errorf("can't get service config for resource %v: %w", imageName, err)
		}
		timeout = src.Timeout + timeout
		return Action{
			Todo:   a,
			Parent: parent,
			Target: res,
			Metadata: &ActionMetadata{
				ServiceTimeout: &timeout,
			},
		}, nil
	}
	return Action{
		Todo:   a,
		Parent: parent,
		Target: res,
	}, nil
}
