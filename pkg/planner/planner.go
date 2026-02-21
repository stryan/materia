package planner

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/containers/podman/v5/pkg/systemd/parser"
	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/services"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/plan"
)

type HostStateManager interface {
	ListVolumes(context.Context) ([]*containers.Volume, error)
	GetNetwork(context.Context, string) (*containers.Network, error)
	ListImages(context.Context) ([]*containers.Image, error)
	ListNetworks(context.Context) ([]*containers.Network, error)
	GetVolume(context.Context, string) (*containers.Volume, error)
	GetService(context.Context, string) (*services.Service, error)
}

type Planner struct {
	PlannerConfig
	Host HostStateManager
}

func NewPlanner(conf PlannerConfig, host HostStateManager) *Planner {
	return &Planner{conf, host}
}

func (p *Planner) Plan(ctx context.Context, hostname string, installedComponents, assignedComponents []*components.Component) (*plan.Plan, error) {
	installedCompNames := make([]string, 0, len(installedComponents))
	for _, c := range installedComponents {
		installedCompNames = append(installedCompNames, c.Name)
	}
	actionPlan := plan.NewPlan()
	componentGraph, err := BuildComponentGraph(ctx, installedComponents, assignedComponents)
	if err != nil {
		return nil, err
	}

	for _, currentTree := range componentGraph.List() {
		if currentTree.Host == nil {
			currentTree.Source.State = components.StateFresh
			actions, err := p.PlanFreshComponent(ctx, currentTree)
			if err != nil {
				return nil, err
			}
			err = actionPlan.Append(actions)
			if err != nil {
				return nil, fmt.Errorf("error calculating fresh component %v differences:%w", currentTree.Name, err)
			}
		} else if currentTree.Source == nil {
			currentTree.Host.State = components.StateNeedRemoval
			actions, err := p.PlanRemovedComponent(ctx, currentTree)
			if err != nil {
				return nil, err
			}
			err = actionPlan.Append(actions)
			if err != nil {
				return nil, fmt.Errorf("error calculating removed component %v differences:%w", currentTree.Name, err)
			}
		} else {
			currentTree.Host.State = components.StateMayNeedUpdate
			currentTree.Source.State = components.StateFresh
			actions, err := p.PlanUpdatedComponent(ctx, currentTree)
			if err != nil {
				return nil, err
			}
			err = actionPlan.Append(actions)
			if err != nil {
				return nil, fmt.Errorf("error calculating updated component %v differences:%w", currentTree.Name, err)
			}
		}
	}
	planValidationPipeline := plan.NewDefaultValidationPipeline(installedCompNames)
	if err := planValidationPipeline.Validate(actionPlan); err != nil {
		return nil, fmt.Errorf("generated invalid plan: %w", err)
	}
	return actionPlan, nil
}

func (m *Planner) PlanFreshComponent(ctx context.Context, currentTree *ComponentTree) ([]actions.Action, error) {
	resourceActions, err := generateFreshComponentResources(currentTree.Source)
	if err != nil {
		return nil, fmt.Errorf("can't generate fresh resources for %v: %w", currentTree.Name, err)
	}
	ensureActions, err := generateQuadletEnsurements(ctx, m.Host, currentTree.Source)
	if err != nil {
		return nil, fmt.Errorf("can't ensure resources: %w", err)
	}
	resourceActions = append(resourceActions, ensureActions...)
	if m.OnlyResources {
		return resourceActions, nil
	}
	if len(resourceActions) > 0 {
		resourceActions = append(resourceActions, actions.Action{
			Todo:   actions.ActionReload,
			Parent: components.NewRootComponent(),
			Target: components.Resource{Kind: components.ResourceTypeHost},
		})
	}
	serviceActions, err := processFreshOrUnchangedComponentServices(ctx, m.Host, currentTree.Source)
	if err != nil {
		return nil, fmt.Errorf("can't plan fresh services for %v: %w", currentTree.Name, err)
	}
	if currentTree.Source.SetupScript != "" {
		c := currentTree.Source
		setupResource, err := c.Resources.Get(c.SetupScript)
		if err != nil {
			return nil, fmt.Errorf("setup resource %v not found: %w", c.SetupScript, err)
		}
		serviceActions = append(serviceActions, actions.Action{
			Todo:     actions.ActionSetup,
			Parent:   c,
			Target:   setupResource,
			Priority: 5,
		})
	}
	currentTree.FinalState = components.StateFresh
	return append(resourceActions, serviceActions...), nil
}

func (m *Planner) PlanRemovedComponent(ctx context.Context, currentTree *ComponentTree) ([]actions.Action, error) {
	resourceActions, err := generateRemovedComponentResources(ctx, m.Host, m.PlannerConfig, currentTree.Host)
	if err != nil {
		return nil, fmt.Errorf("can't generate removed resources for %v: %w", currentTree.Name, err)
	}
	if m.OnlyResources {
		return resourceActions, nil
	}
	serviceActions, err := processRemovedComponentServices(ctx, m.Host, currentTree.Host)
	if err != nil {
		return nil, fmt.Errorf("can't plan removed services for %v: %w", currentTree.Name, err)
	}
	resourceActions = append(resourceActions, actions.Action{
		Todo:   actions.ActionReload,
		Parent: components.NewRootComponent(),
		Target: components.Resource{Kind: components.ResourceTypeHost},
	})
	if currentTree.Host.CleanupScript != "" {
		c := currentTree.Host
		setupResource, err := c.Resources.Get(c.CleanupScript)
		if err != nil {
			return nil, fmt.Errorf("cleanup resource %v not found: %w", c.CleanupScript, err)
		}
		serviceActions = append(serviceActions, actions.Action{
			Todo:     actions.ActionCleanup,
			Parent:   c,
			Target:   setupResource,
			Priority: 2,
		})
	}
	currentTree.FinalState = components.StateNeedRemoval
	return append(resourceActions, serviceActions...), nil
}

func (m *Planner) PlanUpdatedComponent(ctx context.Context, currentTree *ComponentTree) ([]actions.Action, error) {
	resourceActions, err := generateUpdatedComponentResources(ctx, m.Host, m.PlannerConfig, currentTree.Host, currentTree.Source)
	if err != nil {
		return nil, fmt.Errorf("can't generate resources for %v: %w", currentTree.Name, err)
	}
	if currentTree.Host.Version != components.DefaultComponentVersion {
		currentTree.Host.Version = components.DefaultComponentVersion
		resourceActions = append(resourceActions, actions.Action{
			Todo:   actions.ActionUpdate,
			Parent: currentTree.Host,
			Target: components.Resource{Parent: currentTree.Source.Name, Kind: components.ResourceTypeComponent, Path: currentTree.Source.Name},
		})
	}

	if m.OnlyResources {
		return resourceActions, nil
	}
	ensureActions, err := generateQuadletEnsurements(ctx, m.Host, currentTree.Host)
	if err != nil {
		return nil, fmt.Errorf("can't ensure resources: %w", err)
	}
	resourceActions = append(resourceActions, ensureActions...)
	var serviceActions []actions.Action
	if len(resourceActions) > 0 {
		resourceActions = append(resourceActions, actions.Action{
			Todo:   actions.ActionReload,
			Parent: components.NewRootComponent(),
			Target: components.Resource{Kind: components.ResourceTypeHost},
		})
		currentTree.Host.State = components.StateNeedUpdate
		triggeredActions, err := generateComponentServiceTriggers(currentTree.Source)
		if err != nil {
			return nil, fmt.Errorf("can't generate component service triggers for %v: %w", currentTree.Name, err)
		}
		if !m.OnlyResources {
			serviceActions, err = processUpdatedComponentServices(ctx, m.Host, currentTree.Host, currentTree.Source, resourceActions, triggeredActions)
			if err != nil {
				return nil, fmt.Errorf("can't process updated services for component %v: %w", currentTree.Name, err)
			}
		}
	} else if !m.OnlyResources {
		serviceActions, err = processFreshOrUnchangedComponentServices(ctx, m.Host, currentTree.Host)
		if err != nil {
			return nil, fmt.Errorf("can't plan unchanged services for %v: %w", currentTree.Name, err)
		}
		if len(serviceActions) > 0 {
			currentTree.Host.State = components.StateNeedUpdate
			currentTree.FinalState = components.StateNeedUpdate
		} else {
			currentTree.Host.State = components.StateOK
			currentTree.FinalState = components.StateOK
		}
	}

	return append(resourceActions, serviceActions...), nil
}

func generateFreshComponentResources(comp *components.Component) ([]actions.Action, error) {
	var result []actions.Action
	if err := comp.Validate(); err != nil {
		return result, fmt.Errorf("invalid component %v: %w", comp.Name, err)
	}
	if comp.State != components.StateFresh {
		return result, errors.New("expected fresh component")
	}

	result = append(result, actions.Action{
		Todo:   actions.ActionInstall,
		Parent: comp,
		Target: components.Resource{Parent: comp.Name, Kind: components.ResourceTypeComponent, Path: comp.Name},
	})

	for _, r := range comp.Resources.List() {
		content := ""
		if r.Kind != components.ResourceTypePodmanSecret {
			content = r.Content
		}
		result = append(result, actions.Action{
			Todo:        actions.ActionInstall,
			Parent:      comp,
			Target:      r,
			DiffContent: diffmatchpatch.New().DiffMain("", content, false),
		})
	}
	return result, nil
}

func generateUpdatedComponentResources(ctx context.Context, mgr HostStateManager, opts PlannerConfig, host *components.Component, source *components.Component) ([]actions.Action, error) {
	var diffActions []actions.Action
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
		diffActions = append(diffActions, actions.Action{
			Todo:        actions.ActionRemove,
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
		diffActions = append(diffActions, actions.Action{
			Todo:        actions.ActionInstall,
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
			diffActions = append(diffActions, actions.Action{
				Todo:        actions.ActionUpdate,
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

func generateRemovedComponentResources(ctx context.Context, mgr HostStateManager, opts PlannerConfig, comp *components.Component) ([]actions.Action, error) {
	var result []actions.Action
	if err := comp.Validate(); err != nil {
		return result, fmt.Errorf("invalid component %v: %w", comp.Name, err)
	}
	if comp.State != components.StateNeedRemoval {
		return result, errors.New("expected to be removed component")
	}

	manifestResource, err := comp.Resources.Get(manifests.ComponentManifestFile)
	if err != nil {
		return result, fmt.Errorf("can't get component manifest:%w", err)
	}
	resourceList := comp.Resources.List()
	slices.Reverse(resourceList)
	dirs := []components.Resource{}
	for _, r := range resourceList {
		if r.Path != manifests.ComponentManifestFile {
			if r.IsFile() {
				result = append(result, actions.Action{
					Todo:        actions.ActionRemove,
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
				return result, err
			}
			result = append(result, cleanupActions...)
		}

	}
	for _, d := range dirs {
		result = append(result, actions.Action{
			Todo:   actions.ActionRemove,
			Parent: comp,
			Target: d,
		})
	}
	result = append(result, actions.Action{
		Todo:        actions.ActionRemove,
		Parent:      comp,
		Target:      manifestResource,
		DiffContent: diffmatchpatch.New().DiffMain(manifestResource.Content, "", false),
	})
	result = append(result, actions.Action{
		Todo:   actions.ActionRemove,
		Parent: comp,
		Target: components.Resource{Parent: comp.Name, Kind: components.ResourceTypeComponent, Path: comp.Name},
	})
	return result, nil
}

func generateCleanupResourceActions(ctx context.Context, mgr HostStateManager, opts PlannerConfig, parent *components.Component, res components.Resource) ([]actions.Action, error) {
	var result []actions.Action
	switch res.Kind {
	case components.ResourceTypeNetwork:
		_, err := mgr.GetNetwork(ctx, res.HostObject)
		if errors.Is(err, containers.ErrPodmanObjectNotFound) {
			return result, nil
		}
		if err != nil {
			return result, fmt.Errorf("could not get network %v for cleanup: %v", res.Path, err)
		}
		result = append(result, actions.Action{
			Todo:   actions.ActionCleanup,
			Parent: parent,
			Target: res,
		})
	case components.ResourceTypeBuild, components.ResourceTypeImage:
		images, err := mgr.ListImages(ctx)
		if err != nil {
			return result, err
		}
		for _, i := range images {
			if slices.Contains(i.Names, res.HostObject) {
				result = append(result, actions.Action{
					Todo:   actions.ActionCleanup,
					Parent: parent,
					Target: res,
				})
			}
		}
	case components.ResourceTypeVolume:
		if !opts.CleanupVolumes {
			return result, nil
		}
		_, err := mgr.GetVolume(ctx, res.HostObject)
		if errors.Is(err, containers.ErrPodmanObjectNotFound) {
			return result, nil
		}
		if err != nil {
			return result, fmt.Errorf("could not get volume %v for cleanup: %v", res.Path, err)
		}
		if opts.BackupVolumes {
			result = append(result, actions.Action{
				Todo:   actions.ActionDump,
				Parent: parent,
				Target: res,
			})
		}
		result = append(result, actions.Action{
			Todo:   actions.ActionCleanup,
			Parent: parent,
			Target: res,
		})
	}
	return result, nil
}

func generateComponentServiceTriggers(newComponent *components.Component) (map[string][]actions.Action, error) {
	triggeredActions := make(map[string][]actions.Action)
	for _, src := range newComponent.Services.List() {
		for _, trigger := range src.RestartedBy {
			triggerAction, err := getServiceAction(src, newComponent, actions.ActionRestart)
			if err != nil {
				return triggeredActions, err
			}
			triggeredActions[trigger] = append(triggeredActions[trigger], triggerAction)
		}
		for _, trigger := range src.ReloadedBy {
			triggerAction, err := getServiceAction(src, newComponent, actions.ActionReload)
			if err != nil {
				return triggeredActions, err
			}
			triggeredActions[trigger] = append(triggeredActions[trigger], triggerAction)
		}
	}

	return triggeredActions, nil
}

func generateVolumeMigrationActions(ctx context.Context, mgr HostStateManager, parent *components.Component, volumeRes components.Resource) ([]actions.Action, error) {
	var diffActions []actions.Action
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
		return diffActions, nil
	}
	// volume resource has been updated and volume migration has been enabled, add extra actions
	stoppedServiceActions := []actions.Action{}
	for _, s := range parent.Services.List() {
		// stop all services so that we're safe to dump
		currentServ, err := getLiveService(ctx, mgr, parent, s)
		if err != nil {
			return diffActions, fmt.Errorf("can't get service %v when generating volume migration: %w", s.Service, err)
		}
		if currentServ.State != "active" {
			continue
		}
		stopAction, err := getServiceAction(s, parent, actions.ActionStop)
		if err != nil {
			return diffActions, err
		}
		stopAction.Priority = 1
		diffActions = append(diffActions, stopAction)
		stoppedServiceActions = append(stoppedServiceActions, stopAction)
	}
	diffActions = append(diffActions, actions.Action{
		Todo:     actions.ActionDump,
		Parent:   parent,
		Target:   volumeRes,
		Priority: 1,
	})
	diffActions = append(diffActions, actions.Action{
		Todo:     actions.ActionCleanup,
		Parent:   parent,
		Target:   volumeRes,
		Priority: 2,
	})
	diffActions = append(diffActions, actions.Action{
		Todo:     actions.ActionEnsure,
		Parent:   parent,
		Target:   volumeRes,
		Priority: 4,
	})
	diffActions = append(diffActions, actions.Action{
		Todo:     actions.ActionImport,
		Parent:   parent,
		Target:   volumeRes,
		Priority: 4,
	})
	for _, s := range stoppedServiceActions {
		startAction := s
		startAction.Todo = actions.ActionStart
		startAction.Priority = 5
		diffActions = append(diffActions, startAction)
	}
	return diffActions, nil
}

func serviceActionWithMetadata(parent *components.Component, targetSrc components.Resource, s manifests.ServiceResourceConfig, a actions.ActionType) actions.Action {
	var metadata *actions.ActionMetadata
	if s.Timeout != 0 {
		metadata = &actions.ActionMetadata{
			ServiceTimeout: &s.Timeout,
		}
	}
	return actions.Action{
		Todo:     a,
		Parent:   parent,
		Target:   targetSrc,
		Metadata: metadata,
	}
}

func generateServiceRemovalActions(comp *components.Component, osrc manifests.ServiceResourceConfig) ([]actions.Action, error) {
	var result []actions.Action
	res := components.Resource{
		Parent: comp.Name,
		Path:   osrc.Service,
		Kind:   components.ResourceTypeService,
	}
	if osrc.Static {
		// For now we don't need metadata on Enable/Disable actions since they should be effectively instant
		result = append(result, actions.Action{
			Todo:   actions.ActionDisable,
			Parent: comp,
			Target: res,
		})
	}
	stopAction, err := getServiceAction(osrc, comp, actions.ActionStop)
	if err != nil {
		return result, err
	}
	result = append(result, stopAction)
	return result, nil
}

func generateServiceInstallActions(comp *components.Component, osrc manifests.ServiceResourceConfig, liveService *services.Service) ([]actions.Action, error) {
	var result []actions.Action
	if shouldEnableService(osrc, liveService) {
		// For now we don't need metadata on Enable/Disable actions since they should be effectively instant
		res := components.Resource{
			Parent: comp.Name,
			Path:   osrc.Service,
			Kind:   components.ResourceTypeService,
		}
		result = append(result, actions.Action{
			Todo:   actions.ActionEnable,
			Parent: comp,
			Target: res,
		})
	}
	if !liveService.Started() {
		startAction, err := getServiceAction(osrc, comp, actions.ActionStart)
		if err != nil {
			return result, err
		}
		result = append(result, startAction)
	}
	return result, nil
}

func getLiveService(ctx context.Context, mgr HostStateManager, parent *components.Component, src manifests.ServiceResourceConfig) (*services.Service, error) {
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
	liveService, err := mgr.GetService(ctx, name)
	if err != nil {
		return nil, err
	}
	return liveService, nil
}

func processFreshOrUnchangedComponentServices(ctx context.Context, mgr HostStateManager, component *components.Component) ([]actions.Action, error) {
	var actions []actions.Action
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

func processRemovedComponentServices(ctx context.Context, mgr HostStateManager, comp *components.Component) ([]actions.Action, error) {
	var result []actions.Action
	if comp.Services == nil {
		return result, nil
	}
	for _, s := range comp.Services.List() {
		liveService, err := getLiveService(ctx, mgr, comp, s)
		if errors.Is(err, services.ErrServiceNotFound) {
			continue
		}
		if err != nil {
			return result, fmt.Errorf("can't get live service for %v: %w", s.Service, err)
		}
		if liveService.Started() {
			stopAction, err := getServiceAction(s, comp, actions.ActionStop)
			if err != nil {
				return result, fmt.Errorf("can't generate removal actions for %v: %w", s.Service, err)
			}
			result = append(result, stopAction)
		}
	}
	return result, nil
}

func processUpdatedComponentServices(ctx context.Context, host HostStateManager, original, newComponent *components.Component, resourceActions []actions.Action, triggeredActions map[string][]actions.Action) ([]actions.Action, error) {
	var result []actions.Action
	var triggeredServices []string

	for _, d := range resourceActions {
		if updatedServiceActions, ok := triggeredActions[d.Target.Path]; ok {
			result = append(result, updatedServiceActions...)
			for _, a := range updatedServiceActions {
				triggeredServices = append(triggeredServices, a.Target.Path)
			}
		} else if (d.Target.Kind == components.ResourceTypeContainer || d.Target.Kind == components.ResourceTypePod) && d.Todo == actions.ActionUpdate && !newComponent.Settings.NoRestart {
			restartAction, err := resourceActionWithMetadata(d.Target, newComponent, actions.ActionRestart)
			if err != nil {
				return result, fmt.Errorf("error generating auto-restart option for resource %v: %w", d.Target.Path, err)
			}
			result = append(result, restartAction)
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
		result = append(result, installActions...)
	}

	for _, osrc := range original.Services.List() {
		if !slices.Contains(newComponent.Services.ListServiceNames(), osrc.Service) {
			removalActions, err := generateServiceRemovalActions(original, osrc)
			if err != nil {
				return nil, fmt.Errorf("can't generate removal actions for %v:%w", osrc.Service, err)
			}
			result = append(result, removalActions...)
		}
	}

	return result, nil
}

func shouldEnableService(s manifests.ServiceResourceConfig, liveService *services.Service) bool {
	return !s.Disabled && s.Static && !liveService.Enabled
}

func getServiceAction(src manifests.ServiceResourceConfig, parent *components.Component, a actions.ActionType) (actions.Action, error) {
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

func resourceActionWithMetadata(res components.Resource, parent *components.Component, a actions.ActionType) (actions.Action, error) {
	if !a.IsServiceAction() {
		return actions.Action{}, fmt.Errorf("can't create resource action with metadata from type %v", a)
	}

	if !res.IsQuadlet() && res.Kind != components.ResourceTypeService {
		return actions.Action{}, fmt.Errorf("can't create resource service action from non-quadlet %v", res)
	}

	if res.Kind != components.ResourceTypeContainer && res.Kind != components.ResourceTypePod {
		// TODO volumes can be based off images too and need their timeouts adjusted accordingly
		return actions.Action{
			Todo:   a,
			Parent: parent,
			Target: res,
		}, nil
	}

	if res.Kind == components.ResourceTypePod {
		// TODO figure out how to calculate timeout for pod
		timeout := 60000 // 10 minutes
		return actions.Action{
			Todo:   a,
			Parent: parent,
			Target: res,
			Metadata: &actions.ActionMetadata{
				ServiceTimeout: &timeout,
			},
		}, nil
	}
	unitfile := parser.NewUnitFile()
	err := unitfile.Parse(res.Content)
	if err != nil {
		return actions.Action{}, fmt.Errorf("error parsing systemd unit file: %w", err)
	}
	imageName, ok := unitfile.Lookup("Container", "Image")
	if !ok {
		return actions.Action{}, fmt.Errorf("invalid container quadlet: %v", res)
	}
	if strings.HasSuffix(imageName, ".image") || strings.HasSuffix(imageName, ".build") {
		timeout := 60
		src, err := parent.Services.Get(imageName)
		if errors.Is(err, components.ErrServiceNotFound) {
			// no custom timeout defined
			return actions.Action{
				Todo:   a,
				Parent: parent,
				Target: res,
				Metadata: &actions.ActionMetadata{
					ServiceTimeout: &timeout,
				},
			}, nil
		} else if err != nil {
			return actions.Action{}, fmt.Errorf("can't get service config for resource %v: %w", imageName, err)
		}
		timeout = src.Timeout + timeout
		return actions.Action{
			Todo:   a,
			Parent: parent,
			Target: res,
			Metadata: &actions.ActionMetadata{
				ServiceTimeout: &timeout,
			},
		}, nil
	}
	return actions.Action{
		Todo:   a,
		Parent: parent,
		Target: res,
	}, nil
}

func generateQuadletEnsurements(ctx context.Context, mgr HostStateManager, comp *components.Component) ([]actions.Action, error) {
	var result []actions.Action
	var questionableResources []components.Resource
	for _, r := range comp.Resources.List() {
		if r.Kind == components.ResourceTypeNetwork || r.Kind == components.ResourceTypeVolume {
			questionableResources = append(questionableResources, r)
		}
	}
	if len(questionableResources) == 0 {
		return result, nil
	}

	volumes, err := mgr.ListVolumes(ctx)
	if err != nil {
		return result, fmt.Errorf("can't list volumes: %w", err)
	}
	networks, err := mgr.ListNetworks(ctx)
	if err != nil {
		return result, fmt.Errorf("can't list networks: %w", err)
	}

	for _, r := range questionableResources {
		found := false
		serv, err := mgr.GetService(ctx, r.Service())
		if errors.Is(err, services.ErrServiceNotFound) {
			continue
		}
		if err != nil {
			return result, err
		}
		if !serv.Started() {
			continue
		}
		if r.Kind == components.ResourceTypeVolume {
			for _, v := range volumes {
				if v.Name == r.HostObject {
					found = true
					break
				}
			}
		}
		if r.Kind == components.ResourceTypeNetwork {
			for _, n := range networks {
				if n.Name == r.HostObject {
					found = true
					break
				}
			}
		}
		if found {
			continue
		}
		result = append(result, actions.Action{
			Todo:   actions.ActionEnsure,
			Parent: comp,
			Target: r,
		})
	}

	return result, nil
}
