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
	plan := NewPlan(m.Facts)
	var err error

	if len(m.Facts.GetInstalledComponents()) == 0 && len(m.Facts.GetAssignedComponents()) == 0 {
		return plan, nil
	}

	var newComponents map[string]*components.Component
	log.Debug("calculating component differences")
	currentComponentNames := m.Facts.GetInstalledComponents()
	newComponentNames := m.Facts.GetAssignedComponents()
	currentComponents := make(map[string]*components.Component)
	newComponents2 := make(map[string]*components.Component)
	for _, v := range currentComponentNames {
		comp, err := m.CompRepo.GetComponent(v)
		if err != nil {
			return plan, fmt.Errorf("error loading current components: %w", err)
		}
		currentComponents[comp.Name] = comp
	}
	for _, v := range newComponentNames {
		comp, err := m.SourceRepo.GetComponent(v)
		if err != nil {
			return plan, fmt.Errorf("error loading new components: %w", err)
		}
		newComponents2[comp.Name] = comp
	}
	newComponents, err = m.updateComponents2(newComponents2, currentComponents)
	if err != nil {
		return plan, fmt.Errorf("error determining components: %w", err)
	}
	// Determine diff actions
	log.Debug("calculating resource differences")
	finalComponents, err := m.calculateDiffs(ctx, currentComponents, newComponents, plan)
	if err != nil {
		return plan, fmt.Errorf("error calculating diffs: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("generated invalid plan: %w", err)
	}
	var installing, removing, updating, ok []string
	keys := sortedKeys(finalComponents)
	for _, k := range keys {
		v := finalComponents[k]
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

func (m *Materia) updateComponents2(assignedComponents map[string]*components.Component, installedComponents map[string]*components.Component) (map[string]*components.Component, error) {
	componentDiffs := make(map[string]*components.Component)
	for _, v := range assignedComponents {
		if old, ok := installedComponents[v.Name]; !ok {
			v.State = components.StateFresh
			componentDiffs[v.Name] = v
		} else {
			old.State = components.StateMayNeedUpdate
			componentDiffs[old.Name] = old
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

func (m *Materia) calculateDiffs(ctx context.Context, oldComps, updates map[string]*components.Component, plan *Plan) (map[string]*components.Component, error) {
	keys := sortedKeys(updates)
	for _, compName := range keys {
		needUpdate := false
		newComponent := updates[compName]
		if err := newComponent.Validate(); err != nil {
			return nil, err
		}
		vars := m.Secrets.Lookup(ctx, secrets.SecretFilter{
			Hostname:  m.Facts.GetHostname(),
			Roles:     m.Facts.GetRoles(),
			Component: newComponent.Name,
		})
		var err error
		switch newComponent.State {
		case components.StateFresh:
			needUpdate, err = m.calculateFreshComponent(ctx, newComponent, vars, plan)
			if err != nil {
				return nil, fmt.Errorf("can't process fresh component %v: %w", newComponent.Name, err)
			}

		case components.StateMayNeedUpdate:
			original, ok := oldComps[compName]
			if !ok {
				return nil, fmt.Errorf("enable to calculate component diff for %v: could not get installed component", compName)
			}
			needUpdate, err = m.calculatePotentialComponent(ctx, original, newComponent, vars, plan)
			if err != nil {
				return nil, fmt.Errorf("can't process updates for component %v: %w", newComponent.Name, err)
			}
		case components.StateStale, components.StateNeedRemoval:
			needUpdate, err = m.calculateRemoval(ctx, newComponent, plan)
			if err != nil {
				return nil, fmt.Errorf("can't process to be removed component %v: %w", newComponent.Name, err)
			}
		case components.StateRemoved:
			continue
		case components.StateUnknown:
			return nil, errors.New("found unknown component")
		default:
			panic(fmt.Sprintf("unexpected main.ComponentLifecycle: %#v", newComponent.State))
		}
		if needUpdate {
			plan.Add(Action{
				Todo:   ActionReloadUnits,
				Parent: m.rootComponent,
			})
		}
	}

	return updates, nil
}

func (m *Materia) calculateFreshComponent(ctx context.Context, newComponent *components.Component, vars map[string]any, plan *Plan) (bool, error) {
	plan.Add(Action{
		Todo:   ActionInstallComponent,
		Parent: newComponent,
	})
	maps.Copy(vars, newComponent.Defaults)
	for _, r := range newComponent.Resources {
		// do a test run just to make sure we can actually install this resource
		newStringTempl, err := m.SourceRepo.ReadResource(r)
		if err != nil {
			return false, err
		}
		_, err = m.executeResource(newStringTempl, vars)
		if err != nil {
			return false, err
		}

		plan.Add(Action{
			Todo:    resToAction(r, "install"),
			Parent:  newComponent,
			Payload: r,
		})
	}
	if newComponent.Scripted {
		plan.Add(Action{
			Todo:   ActionSetupComponent,
			Parent: newComponent,
		})
	}
	if m.onlyResources {
		return true, nil
	}
	sortedSrcs := sortedKeys(newComponent.ServiceResources)
	for _, k := range sortedSrcs {
		s := newComponent.ServiceResources[k]
		res := components.Resource{
			Parent: newComponent.Name,
			Name:   k,
			Kind:   components.ResourceTypeService,
		}
		liveService, err := m.Services.Get(ctx, k)
		if errors.Is(err, services.ErrServiceNotFound) {
			liveService = &services.Service{
				Name:    k,
				State:   "non-existent",
				Enabled: false,
			}
		} else if err != nil {
			return false, err
		}
		if m.shouldEnableService(s, liveService) {
			plan.Add(Action{
				Todo:    ActionEnableService,
				Parent:  newComponent,
				Payload: res,
			})
		}
		if !liveService.Started() {
			plan.Add(Action{
				Todo:    ActionStartService,
				Parent:  newComponent,
				Payload: res,
			})
		}
	}
	return true, nil
}

func (m *Materia) calculatePotentialComponent(ctx context.Context, original, newComponent *components.Component, vars map[string]any, plan *Plan) (bool, error) {
	needUpdate := false
	resourceActions, err := m.diffComponent(original, newComponent, vars)
	if err != nil {
		log.Debugf("error diffing components: L (%v) R (%v)", original, newComponent)
		return false, err
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
	if len(resourceActions) != 0 {
		newComponent.State = components.StateNeedUpdate
		needUpdate = true
		for _, d := range resourceActions {
			plan.Add(d)
			if updatedService, ok := restartmap[d.Payload.Name]; ok {
				plan.Add(Action{
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
				plan.Add(Action{
					Todo:   ActionReloadService,
					Parent: newComponent,
					Payload: components.Resource{
						Parent: newComponent.Name,
						Name:   updatedService.Service,
						Kind:   components.ResourceTypeService,
					},
				})
			}
			if m.diffs && d.Category() == ActionCategoryUpdate {
				diffs := d.Content.([]diffmatchpatch.Diff)
				fmt.Printf("Diffs:\n%v", diffmatchpatch.New().DiffPrettyText(diffs))
			}

		}
		if m.onlyResources {
			newComponent.State = components.StateOK
			return needUpdate, nil
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
			res := components.Resource{
				Parent: newComponent.Name,
				Name:   k,
				Kind:   components.ResourceTypeService,
			}
			liveService, err := m.Services.Get(ctx, k)
			if errors.Is(err, services.ErrServiceNotFound) {
				liveService = &services.Service{
					Name:    k,
					State:   "non-existent",
					Enabled: false,
				}
			} else if err != nil {
				return false, err
			}
			if m.shouldEnableService(s, liveService) {
				plan.Add(Action{
					Todo:    ActionEnableService,
					Parent:  newComponent,
					Payload: res,
				})
			}
			if !liveService.Started() {
				plan.Add(Action{
					Todo:    ActionStartService,
					Parent:  newComponent,
					Payload: res,
				})
			}
		}
		sortedOldSrcs := sortedKeys(original.ServiceResources)
		for _, osrc := range sortedOldSrcs {
			s := original.ServiceResources[newComponent.Name]
			if !slices.Contains(sortedSrcs, osrc) {
				// service is no longer managed by materia, stop it
				res := components.Resource{
					Parent: original.Name,
					Name:   osrc,
					Kind:   components.ResourceTypeService,
				}
				if s.Static {
					plan.Add(Action{
						Todo:    ActionDisableService,
						Parent:  newComponent,
						Payload: res,
					})
				}
				plan.Add(Action{
					Todo:    ActionStopService,
					Parent:  newComponent,
					Payload: res,
				})
			}
		}
	} else if !m.onlyResources {
		serviceChanged := false
		for _, s := range newComponent.ServiceResources {
			liveService, err := m.Services.Get(ctx, s.Service)
			if errors.Is(err, services.ErrServiceNotFound) {
				liveService = &services.Service{
					Name:    s.Service,
					State:   "non-existent",
					Enabled: false,
				}
			} else if err != nil {
				return false, err
			}
			res := components.Resource{
				Parent: newComponent.Name,
				Name:   s.Service,
				Kind:   components.ResourceTypeService,
			}

			if m.shouldEnableService(s, liveService) {
				serviceChanged = true
				plan.Add(Action{
					Todo:    ActionEnableService,
					Parent:  newComponent,
					Payload: res,
				})
			}
			if !liveService.Started() {
				serviceChanged = true
				plan.Add(Action{
					Todo:    ActionStartService,
					Parent:  newComponent,
					Payload: res,
				})
			}

		}
		if !serviceChanged {
			newComponent.State = components.StateOK
		} else {
			newComponent.State = components.StateNeedUpdate
		}
	} else {
		newComponent.State = components.StateOK
	}
	return needUpdate, nil
}

func (m *Materia) calculateRemoval(ctx context.Context, comp *components.Component, plan *Plan) (bool, error) {
	for _, r := range comp.Resources {
		plan.Add(Action{
			Todo:    resToAction(r, "remove"),
			Parent:  comp,
			Payload: r,
		})
	}
	if comp.Scripted {
		plan.Add(Action{
			Todo:   ActionCleanupComponent,
			Parent: comp,
		})
	}
	if !m.onlyResources {
		for _, s := range comp.ServiceResources {
			res := components.Resource{
				Parent: comp.Name,
				Name:   s.Service,
				Kind:   components.ResourceTypeService,
			}
			liveService, err := m.Services.Get(ctx, comp.Name)
			if err != nil {
				return false, err
			}
			if liveService.Started() {
				plan.Add(Action{
					Todo:    ActionStopService,
					Parent:  comp,
					Payload: res,
				})
			}
		}
	}
	plan.Add(Action{
		Todo:   ActionRemoveComponent,
		Parent: comp,
	})
	return true, nil
}

func (m *Materia) diffComponent(base, other *components.Component, vars map[string]any) ([]Action, error) {
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
			diffs, err := m.diffResource(cur, newRes, diffVars)
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

func (m *Materia) shouldEnableService(s manifests.ServiceResourceConfig, liveService *services.Service) bool {
	return !s.Disabled && s.Static && !liveService.Enabled
}

func (m *Materia) diffResource(cur, newRes components.Resource, vars map[string]any) ([]diffmatchpatch.Diff, error) {
	dmp := diffmatchpatch.New()
	var diffs []diffmatchpatch.Diff
	if err := cur.Validate(); err != nil {
		return diffs, fmt.Errorf("self resource invalid during comparison: %w", err)
	}
	if err := newRes.Validate(); err != nil {
		return diffs, fmt.Errorf("other resource invalid during comparison: %w", err)
	}
	curString, err := m.CompRepo.ReadResource(cur)
	if err != nil {
		return diffs, err
	}
	newStringTempl, err := m.SourceRepo.ReadResource(newRes)
	if err != nil {
		return diffs, err
	}
	var newString string
	result, err := m.executeResource(newStringTempl, vars)
	if err != nil {
		return diffs, err
	}
	newString = result.String()
	return dmp.DiffMain(curString, newString, false), nil
}

func (m *Materia) executeResource(resourceTemplate string, vars map[string]any) (*bytes.Buffer, error) {
	result := bytes.NewBuffer([]byte{})
	tmpl, err := template.New("resource").Option("missingkey=error").Funcs(m.macros(vars)).Parse(resourceTemplate)
	if err != nil {
		return nil, err
	}
	err = tmpl.Execute(result, vars)
	if err != nil {
		return nil, err
	}
	return result, nil
}
