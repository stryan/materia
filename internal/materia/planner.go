package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"text/template"

	"git.saintnet.tech/stryan/materia/internal/components"
	"git.saintnet.tech/stryan/materia/internal/manifests"
	"git.saintnet.tech/stryan/materia/internal/secrets"
	"git.saintnet.tech/stryan/materia/internal/services"
	"github.com/charmbracelet/log"
	"github.com/sergi/go-diff/diffmatchpatch"
)

func (m *Materia) Plan(ctx context.Context) (*Plan, error) {
	currentVolumes, err := m.Containers.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}
	var vollist []string
	for _, v := range currentVolumes {
		vollist = append(vollist, v.Name)
	}
	plan := NewPlan(m.InstalledComponents, vollist)

	if len(m.InstalledComponents) == 0 && len(m.AssignedComponents) == 0 {
		return plan, nil
	}

	var updatedComponents map[string]*components.Component
	log.Debug("calculating component differences")
	currentComponents := make(map[string]*components.Component)
	newComponents := make(map[string]*components.Component)
	for _, v := range m.InstalledComponents {
		comp, err := m.CompRepo.GetComponent(v)
		if err != nil {
			return plan, fmt.Errorf("error loading current components: %w", err)
		}
		currentComponents[comp.Name] = comp
	}
	for _, v := range m.AssignedComponents {
		comp, err := m.SourceRepo.GetComponent(v)
		if err != nil {
			return plan, fmt.Errorf("error loading new components: %w", err)
		}
		newComponents[comp.Name] = comp
	}
	updatedComponents, err = updateComponents(newComponents, currentComponents)
	if err != nil {
		return plan, fmt.Errorf("error determining components: %w", err)
	}
	// Determine diff actions
	log.Debug("calculating resource differences")
	plannedActions, err := m.calculateDiffs(ctx, currentComponents, updatedComponents)
	if err != nil {
		return plan, fmt.Errorf("error calculating diffs: %w", err)
	}
	plan.Append(plannedActions)
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("generated invalid plan: %w", err)
	}
	var installing, removing, updating, ok []string
	keys := sortedKeys(updatedComponents)
	for _, k := range keys {
		v := updatedComponents[k]
		switch v.State {
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
			panic(fmt.Sprintf("unexpected main.ComponentLifecycle: %#v", v.State))
		}
	}

	log.Debug("installing components", "installing", installing)
	log.Debug("removing components", "removing", removing)
	log.Debug("updating components", "updating", updating)
	log.Debug("unchanged components", "unchanged", ok)

	return plan, nil
}

func updateComponents(assignedComponents map[string]*components.Component, installedComponents map[string]*components.Component) (map[string]*components.Component, error) {
	componentDiffs := make(map[string]*components.Component)
	// TODO consider replacing with action based results?
	for _, v := range installedComponents {
		if v.State != components.StateStale {
			return nil, errors.New("installed component isn't stale")
		}
	}
	for _, v := range assignedComponents {
		if v.State != components.StateFresh {
			return nil, errors.New("assigned component isn't fresh")
		}
		if old, ok := installedComponents[v.Name]; !ok {
			v.State = components.StateFresh
			componentDiffs[v.Name] = v
		} else {
			old.State = components.StateMayNeedUpdate
			v.State = components.StateMayNeedUpdate
			componentDiffs[v.Name] = v
		}
	}
	for _, v := range installedComponents {
		if v.State == components.StateStale {
			v.State = components.StateNeedRemoval
			componentDiffs[v.Name] = v
		}
	}

	return componentDiffs, nil
}

func (m *Materia) calculateDiffs(ctx context.Context, oldComps, updates map[string]*components.Component) ([]Action, error) {
	keys := sortedKeys(updates)
	hostname := m.HostFacts.GetHostname()
	var plannedActions []Action
	for _, compName := range keys {
		newComponent := updates[compName]
		if err := newComponent.Validate(); err != nil {
			return plannedActions, err
		}
		vars := m.Secrets.Lookup(ctx, secrets.SecretFilter{
			Hostname:  hostname,
			Roles:     m.Roles,
			Component: newComponent.Name,
		})

		switch newComponent.State {
		case components.StateFresh:
			actions, err := calculateFreshComponent(m.SourceRepo, newComponent, vars, m.macros)
			if err != nil {
				return plannedActions, fmt.Errorf("can't process fresh component %v: %w", newComponent.Name, err)
			}
			if !m.onlyResources {
				if len(actions) > 0 {
					actions = append(actions, Action{
						Todo:    ActionReloadUnits,
						Parent:  RootComponent,
						Payload: components.Resource{},
					})
				}
				sortedSrcs := sortedKeys(newComponent.ServiceResources)
				for _, k := range sortedSrcs {
					s := newComponent.ServiceResources[k]
					installActions, err := generateServiceInstallActions(ctx, newComponent, s, m.Services)
					if err != nil {
						return actions, err
					}
					actions = append(actions, installActions...)
				}
			}

			plannedActions = append(plannedActions, actions...)

		case components.StateMayNeedUpdate:
			original, ok := oldComps[compName]
			if !ok {
				return plannedActions, fmt.Errorf("enable to calculate component diff for %v: could not get installed component", compName)
			}
			actions, err := calculatePotentialComponent(ctx, m.SourceRepo, m.CompRepo, original, newComponent, vars, m.macros, m.Services, m.onlyResources, m.diffs)
			if err != nil {
				return plannedActions, fmt.Errorf("can't process updates for component %v: %w", newComponent.Name, err)
			}

			plannedActions = append(plannedActions, actions...)
		case components.StateStale, components.StateNeedRemoval:
			actions, err := calculateRemoval(ctx, newComponent, m.Services, m.onlyResources)
			if err != nil {
				return plannedActions, fmt.Errorf("can't process to be removed component %v: %w", newComponent.Name, err)
			}

			plannedActions = append(plannedActions, actions...)
		case components.StateRemoved:
			continue
		case components.StateUnknown:
			return plannedActions, errors.New("found unknown component")
		default:
			panic(fmt.Sprintf("unexpected main.ComponentLifecycle: %#v", newComponent.State))
		}

	}

	return plannedActions, nil
}

func calculateFreshComponent(sourceRepo ComponentRepository, newComponent *components.Component, vars map[string]any, macros MacroMap) ([]Action, error) {
	var actions []Action
	actions = append(actions, Action{
		Todo:   ActionInstallComponent,
		Parent: newComponent,
	})
	maps.Copy(vars, newComponent.Defaults)
	for _, r := range newComponent.Resources {
		// do a test run just to make sure we can actually install this resource
		newStringTempl, err := sourceRepo.ReadResource(r)
		if err != nil {
			return actions, err
		}
		_, err = executeResource(newStringTempl, vars, macros)
		if err != nil {
			return actions, err
		}

		actions = append(actions, Action{
			Todo:    resToAction(r, "install"),
			Parent:  newComponent,
			Payload: r,
		})
	}
	if newComponent.Scripted {
		actions = append(actions, Action{
			Todo:   ActionSetupComponent,
			Parent: newComponent,
		})
	}
	return actions, nil
}

func calculatePotentialComponent(ctx context.Context, sourceRepo, compRepo ComponentRepository, original, newComponent *components.Component, vars map[string]any, macros MacroMap, sm Services, onlyResources, diffs bool) ([]Action, error) {
	var actions []Action
	resourceActions, err := diffComponent(sourceRepo, compRepo, original, newComponent, vars, macros)
	if err != nil {
		log.Debugf("error diffing components: L (%v) R (%v)", original, newComponent)
		return actions, err
	}
	actions = append(actions, resourceActions...)
	if onlyResources {
		newComponent.State = components.StateOK
		return actions, nil
	}
	restartmap := make(map[string]manifests.ServiceResourceConfig)
	reloadmap := make(map[string]manifests.ServiceResourceConfig)
	for _, src := range newComponent.ServiceResources {
		for _, trigger := range src.RestartedBy {
			restartmap[trigger] = src
		}
		for _, trigger := range src.ReloadedBy {
			reloadmap[trigger] = src
		}
	}
	var serviceActions []Action
	if len(resourceActions) != 0 {
		if len(actions) > 0 {
			actions = append(actions, Action{
				Todo:    ActionReloadUnits,
				Parent:  RootComponent,
				Payload: components.Resource{},
			})
		}
		newComponent.State = components.StateNeedUpdate
		actions, err := calculateServicesFromUpdate(ctx, original, newComponent, resourceActions, restartmap, reloadmap, sm, diffs)
		if err != nil {
			return actions, err
		}
		serviceActions = append(serviceActions, actions...)
	} else if !onlyResources {
		actions, err := calculateServicesFromUnchanged(ctx, newComponent, sm)
		if err != nil {
			return actions, err
		}
		serviceActions = append(serviceActions, actions...)
	}
	if len(serviceActions) > 0 {
		newComponent.State = components.StateNeedUpdate
	} else {
		newComponent.State = components.StateOK
	}
	actions = append(actions, serviceActions...)
	return actions, nil
}

func calculateServicesFromUpdate(ctx context.Context, original, newComponent *components.Component, resourceActions []Action, restartmap, reloadmap map[string]manifests.ServiceResourceConfig, sm Services, diffs bool) ([]Action, error) {
	var actions []Action
	newComponent.State = components.StateNeedUpdate
	for _, d := range resourceActions {
		if updatedService, ok := restartmap[d.Payload.Name]; ok {
			actions = append(actions, Action{
				Todo:   ActionRestartService,
				Parent: newComponent,
				Payload: components.Resource{
					Parent: newComponent.Name,
					Name:   updatedService.Service,
					Kind:   components.ResourceTypeService,
				},
			})
		}
		if updatedService, ok := reloadmap[d.Payload.Name]; ok {
			actions = append(actions, Action{
				Todo:   ActionReloadService,
				Parent: newComponent,
				Payload: components.Resource{
					Parent: newComponent.Name,
					Name:   updatedService.Service,
					Kind:   components.ResourceTypeService,
				},
			})
		}
		if diffs && d.Category() == ActionCategoryUpdate {
			diffs := d.Content.([]diffmatchpatch.Diff)
			fmt.Printf("Diffs:\n%v", diffmatchpatch.New().DiffPrettyText(diffs))
		}

	}
	sortedSrcs := sortedKeys(newComponent.ServiceResources)
	for _, k := range sortedSrcs {
		// skip services that are triggered
		if _, ok := reloadmap[k]; ok {
			continue
		}
		if _, ok := restartmap[k]; ok {
			continue
		}
		s := newComponent.ServiceResources[k]
		installActions, err := generateServiceInstallActions(ctx, newComponent, s, sm)
		if err != nil {
			return nil, err
		}
		actions = append(actions, installActions...)

	}
	sortedOldSrcs := sortedKeys(original.ServiceResources)
	for _, osrc := range sortedOldSrcs {
		s := original.ServiceResources[osrc]
		if !slices.Contains(sortedSrcs, osrc) {
			// service is no longer managed by materia, stop it
			actions = append(actions, generateServiceRemovalActions(original, s)...)
		}
	}
	return actions, nil
}

func calculateServicesFromUnchanged(ctx context.Context, comp *components.Component, sm Services) ([]Action, error) {
	var actions []Action
	for _, s := range comp.ServiceResources {
		installActions, err := generateServiceInstallActions(ctx, comp, s, sm)
		if err != nil {
			return actions, nil
		}
		actions = append(actions, installActions...)
	}

	return actions, nil
}

func generateServiceRemovalActions(comp *components.Component, osrc manifests.ServiceResourceConfig) []Action {
	var result []Action
	res := components.Resource{
		Parent: comp.Name,
		Name:   osrc.Service,
		Kind:   components.ResourceTypeService,
	}
	if osrc.Static {
		result = append(result, Action{
			Todo:    ActionDisableService,
			Parent:  comp,
			Payload: res,
		})
	}
	result = append(result, Action{
		Todo:    ActionStopService,
		Parent:  comp,
		Payload: res,
	})
	return result
}

func generateServiceInstallActions(ctx context.Context, comp *components.Component, osrc manifests.ServiceResourceConfig, sm Services) ([]Action, error) {
	var actions []Action
	liveService, err := sm.Get(ctx, osrc.Service)
	if errors.Is(err, services.ErrServiceNotFound) {
		liveService = &services.Service{
			Name:    osrc.Service,
			State:   "non-existent",
			Enabled: false,
		}
	} else if err != nil {
		return actions, err
	}
	res := components.Resource{
		Parent: comp.Name,
		Name:   osrc.Service,
		Kind:   components.ResourceTypeService,
	}
	if shouldEnableService(osrc, liveService) {
		actions = append(actions, Action{
			Todo:    ActionEnableService,
			Parent:  comp,
			Payload: res,
		})
	}
	if !liveService.Started() {
		actions = append(actions, Action{
			Todo:    ActionStartService,
			Parent:  comp,
			Payload: res,
		})
	}
	return actions, nil
}

func calculateRemoval(ctx context.Context, comp *components.Component, sm Services, onlyResources bool) ([]Action, error) {
	var actions []Action
	for _, r := range comp.Resources {
		actions = append(actions, Action{
			Todo:    resToAction(r, "remove"),
			Parent:  comp,
			Payload: r,
		})
	}
	if comp.Scripted {
		actions = append(actions, Action{
			Todo:   ActionCleanupComponent,
			Parent: comp,
		})
	}
	actions = append(actions, Action{
		Todo:   ActionRemoveComponent,
		Parent: comp,
	})
	if onlyResources {
		return actions, nil
	}
	for _, s := range comp.ServiceResources {
		res := components.Resource{
			Parent: comp.Name,
			Name:   s.Service,
			Kind:   components.ResourceTypeService,
		}
		liveService, err := sm.Get(ctx, comp.Name)
		if err != nil {
			return actions, err
		}
		if liveService.Started() {
			actions = append(actions, Action{
				Todo:    ActionStopService,
				Parent:  comp,
				Payload: res,
			})
		}
	}
	return actions, nil
}

func diffComponent(sourceRepo, compRepo ComponentRepository, base, other *components.Component, vars map[string]any, macros MacroMap) ([]Action, error) {
	var diffActions []Action
	if len(other.Resources) == 0 {
		log.Debug("components", "left", base, "right", other)
		return diffActions, fmt.Errorf("candidate component is missing resources: L:%v R:%v", len(base.Resources), len(other.Resources))
	}
	if err := base.Validate(); err != nil {
		return diffActions, fmt.Errorf("self component invalid during comparison: %w", err)
	}
	if err := other.Validate(); err != nil {
		return diffActions, fmt.Errorf("other component invalid during comparison: %w", err)
	}
	currentResources := make(map[string]components.Resource)
	newResources := make(map[string]components.Resource)
	diffVars := make(map[string]any)
	maps.Copy(diffVars, base.Defaults)
	maps.Copy(diffVars, other.Defaults)
	maps.Copy(diffVars, vars)
	for _, v := range base.Resources {
		currentResources[v.Name] = v
	}
	for _, v := range other.Resources {
		newResources[v.Name] = v
	}

	keys := sortedKeys(currentResources)
	for _, k := range keys {
		cur := currentResources[k]
		if newRes, ok := newResources[k]; ok {
			// check for diffs and update
			log.Debug("diffing resource", "component", base.Name, "file", cur.Name)
			diffs, err := diffResource(sourceRepo, compRepo, cur, newRes, diffVars, macros)
			if err != nil {
				return diffActions, err
			}
			if len(diffs) < 1 {
				// comparing empty files
				continue
			}
			if len(diffs) > 1 || diffs[0].Type != diffmatchpatch.DiffEqual {
				log.Debug("updating current resource", "file", cur.Name, "diffs", diffs)
				a := Action{
					Todo:    resToAction(newRes, "update"),
					Parent:  base,
					Payload: newRes,
					Content: diffs,
				}

				diffActions = append(diffActions, a)
			}
		} else {
			// in current resources but not source resources, remove old
			log.Debug("removing current resource", "file", cur.Name)
			a := Action{
				Todo:    resToAction(newRes, "remove"),
				Parent:  base,
				Payload: cur,
			}

			diffActions = append(diffActions, a)
		}
	}
	keys = sortedKeys(newResources)
	for _, k := range keys {
		if _, ok := currentResources[k]; !ok {
			// if new resource is not in old resource we need to install it
			fmt.Printf("Creating new resource %v", k)
			a := Action{
				Todo:    resToAction(newResources[k], "install"),
				Parent:  base,
				Payload: newResources[k],
			}
			diffActions = append(diffActions, a)
		}
	}

	return diffActions, nil
}

func shouldEnableService(s manifests.ServiceResourceConfig, liveService *services.Service) bool {
	return !s.Disabled && s.Static && !liveService.Enabled
}

func diffResource(sourceRepo, compRepo ComponentRepository, cur, newRes components.Resource, vars map[string]any, macros MacroMap) ([]diffmatchpatch.Diff, error) {
	dmp := diffmatchpatch.New()
	var diffs []diffmatchpatch.Diff
	if err := cur.Validate(); err != nil {
		return diffs, fmt.Errorf("self resource invalid during comparison: %w", err)
	}
	if err := newRes.Validate(); err != nil {
		return diffs, fmt.Errorf("other resource invalid during comparison: %w", err)
	}
	curString, err := compRepo.ReadResource(cur)
	if err != nil {
		return diffs, err
	}
	newStringTempl, err := sourceRepo.ReadResource(newRes)
	if err != nil {
		return diffs, err
	}
	var newString string
	result, err := executeResource(newStringTempl, vars, macros)
	if err != nil {
		return diffs, err
	}
	newString = result.String()
	return dmp.DiffMain(curString, newString, false), nil
}

func executeResource(resourceTemplate string, vars map[string]any, macros MacroMap) (*bytes.Buffer, error) {
	result := bytes.NewBuffer([]byte{})
	tmpl, err := template.New("resource").Option("missingkey=error").Funcs(macros(vars)).Parse(resourceTemplate)
	if err != nil {
		return nil, err
	}
	err = tmpl.Execute(result, vars)
	if err != nil {
		return nil, err
	}
	return result, nil
}
