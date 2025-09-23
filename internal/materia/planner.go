package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"text/template"

	"github.com/charmbracelet/log"
	"github.com/containers/podman/v5/pkg/systemd/parser"
	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/internal/components"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/manifests"
	"primamateria.systems/materia/internal/secrets"
	"primamateria.systems/materia/internal/services"
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
			actions, err := m.calculateFreshComponentResources(newComponent, vars)
			if err != nil {
				return plannedActions, fmt.Errorf("can't process fresh component %v: %w", newComponent.Name, err)
			}
			if len(actions) > 0 {
				actions = append(actions, Action{
					Todo:    ActionReload,
					Parent:  rootComponent,
					Payload: components.Resource{Kind: components.ResourceTypeHost},
				})
			}
			serviceActions, err := m.processFreshComponentServices(ctx, newComponent)
			if err != nil {
				return plannedActions, err
			}
			actions = append(actions, serviceActions...)

			plannedActions = append(plannedActions, actions...)

		case components.StateMayNeedUpdate:
			original, ok := oldComps[compName]
			if !ok {
				return plannedActions, fmt.Errorf("enable to calculate component diff for %v: could not get installed component", compName)
			}

			actions, err := m.calculatePotentialComponentResources(original, newComponent, vars)
			if err != nil {
				return plannedActions, fmt.Errorf("can't process updates for component %v: %w", newComponent.Name, err)
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
			if len(actions) > 0 {
				sactions, err := m.processUpdatedComponentServices(ctx, original, newComponent, actions, restartmap, reloadmap)
				if err != nil {
					return plannedActions, fmt.Errorf("can't process updated services for component %v: %w", compName, err)
				}
				actions = append(actions, sactions...)
			} else {
				sactions, err := m.processUnchangedComponentServices(ctx, newComponent)
				if err != nil {
					return actions, err
				}
				actions = append(actions, sactions...)
			}
			if original.Version != components.DefaultComponentVersion {
				original.Version = components.DefaultComponentVersion
				actions = append(actions, Action{
					Todo:   ActionUpdate,
					Parent: original,
				})
			}
			if len(actions) > 0 {
				newComponent.State = components.StateNeedUpdate
			} else {
				newComponent.State = components.StateOK
			}

			plannedActions = append(plannedActions, actions...)
		case components.StateStale, components.StateNeedRemoval:
			actions, err := m.calculateRemovedComponentResources(newComponent)
			if err != nil {
				return plannedActions, fmt.Errorf("can't process to be removed component %v: %w", newComponent.Name, err)
			}
			serviceActions, err := m.processRemovedComponentServices(ctx, newComponent)
			if err != nil {
				return plannedActions, fmt.Errorf("can't process services for removed component %v: %w", compName, err)
			}
			plannedActions = append(plannedActions, slices.Concat(serviceActions, actions)...)
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

func (m *Materia) calculateFreshComponentResources(newComponent *components.Component, vars map[string]any) ([]Action, error) {
	var actions []Action
	if newComponent.State != components.StateFresh {
		return actions, errors.New("expected fresh component")
	}
	actions = append(actions, Action{
		Todo:    ActionInstall,
		Parent:  newComponent,
		Payload: components.Resource{Parent: newComponent.Name, Kind: components.ResourceTypeComponent, Path: newComponent.Name},
	})
	maps.Copy(vars, newComponent.Defaults)
	for _, r := range newComponent.Resources {
		// do a test run just to make sure we can actually install this resource
		if r.Kind != components.ResourceTypePodmanSecret {
			newStringTempl, err := m.SourceRepo.ReadResource(r)
			if err != nil {
				return actions, err
			}
			resourceBody, err := m.executeResource(newStringTempl, vars)
			if err != nil {
				return actions, err
			}
			if r.IsQuadlet() {
				unitfile := parser.NewUnitFile()
				err = unitfile.Parse(resourceBody.String())
				if err != nil {
					return actions, fmt.Errorf("error parsing container file: %w", err)
				}
				nameOption := ""
				group := ""
				switch r.Kind {
				case components.ResourceTypeContainer:
					group = "Container"
					nameOption = "ContainerName"
				case components.ResourceTypeVolume:
					group = "Volume"
					nameOption = "VolumeName"
				case components.ResourceTypeNetwork:
					group = "Network"
					nameOption = "NetworkName"
				case components.ResourceTypePod:
					group = "Pod"
					nameOption = "PodName"
				}
				if nameOption != "" {
					name, foundName := unitfile.Lookup(group, nameOption)
					if foundName {
						r.HostObject = name
					} else {
						r.HostObject = fmt.Sprintf("systemd-%v", filepath.Base(r.Path))
					}
				}
			}

		}
		actions = append(actions, Action{
			Todo:    ActionInstall,
			Parent:  newComponent,
			Payload: r,
		})
	}
	if newComponent.Scripted {
		actions = append(actions, Action{
			Todo:   ActionSetup,
			Parent: newComponent,
		})
	}
	return actions, nil
}

func (m *Materia) processFreshComponentServices(ctx context.Context, component *components.Component) ([]Action, error) {
	if m.onlyResources {
		return nil, nil
	}

	var actions []Action
	sortedSrcs := sortedKeys(component.ServiceResources)

	for _, k := range sortedSrcs {
		s := component.ServiceResources[k]
		liveService, err := getLiveService(ctx, m.Services, s.Service)
		if err != nil {
			return actions, err
		}

		installActions, err := generateServiceInstallActions(component, s, liveService)
		if err != nil {
			return actions, err
		}
		actions = append(actions, installActions...)
	}

	return actions, nil
}

func (m *Materia) calculatePotentialComponentResources(original, newComponent *components.Component, vars map[string]any) ([]Action, error) {
	var actions []Action
	if newComponent.State != components.StateMayNeedUpdate {
		return actions, fmt.Errorf("expected potential component, got %v", newComponent.State)
	}
	actions, err := m.diffComponent(original, newComponent, vars)
	if err != nil {
		log.Debugf("error diffing components: L (%v) R (%v)", original, newComponent)
		return actions, err
	}
	if len(actions) > 0 {
		actions = append(actions, Action{
			Todo:    ActionReload,
			Parent:  rootComponent,
			Payload: components.Resource{Kind: components.ResourceTypeHost},
		})
	}
	return actions, nil
}

func (m *Materia) processUpdatedComponentServices(ctx context.Context, original, newComponent *components.Component, resourceActions []Action, restartmap, reloadmap map[string]manifests.ServiceResourceConfig) ([]Action, error) {
	var actions []Action
	if m.onlyResources {
		return actions, nil
	}
	for _, d := range resourceActions {
		if m.diffs && d.Todo == ActionUpdate {
			diffs := d.Content.([]diffmatchpatch.Diff)
			fmt.Printf("Diffs:\n%v", diffmatchpatch.New().DiffPrettyText(diffs))
		}
		if updatedService, ok := restartmap[d.Payload.Path]; ok {
			actions = append(actions, Action{
				Todo:   ActionRestart,
				Parent: newComponent,
				Payload: components.Resource{
					Parent: newComponent.Name,
					Path:   updatedService.Service,
					Kind:   components.ResourceTypeService,
				},
			})
			continue // No need to reload if we restart
		}
		if updatedService, ok := reloadmap[d.Payload.Path]; ok {
			actions = append(actions, Action{
				Todo:   ActionReload,
				Parent: newComponent,
				Payload: components.Resource{
					Parent: newComponent.Name,
					Path:   updatedService.Service,
					Kind:   components.ResourceTypeService,
				},
			})
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
		liveService, err := getLiveService(ctx, m.Services, k)
		if err != nil {
			return nil, err
		}

		installActions, err := generateServiceInstallActions(newComponent, s, liveService)
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

func (m *Materia) processUnchangedComponentServices(ctx context.Context, comp *components.Component) ([]Action, error) {
	var actions []Action
	if m.onlyResources {
		return actions, nil
	}
	for _, s := range comp.ServiceResources {
		liveService, err := getLiveService(ctx, m.Services, s.Service)
		if err != nil {
			return nil, err
		}
		installActions, err := generateServiceInstallActions(comp, s, liveService)
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
		Path:   osrc.Service,
		Kind:   components.ResourceTypeService,
	}
	if osrc.Static {
		result = append(result, Action{
			Todo:    ActionDisable,
			Parent:  comp,
			Payload: res,
		})
	}
	result = append(result, Action{
		Todo:    ActionStop,
		Parent:  comp,
		Payload: res,
	})
	return result
}

func generateServiceInstallActions(comp *components.Component, osrc manifests.ServiceResourceConfig, liveService *services.Service) ([]Action, error) {
	var actions []Action
	res := components.Resource{
		Parent: comp.Name,
		Path:   osrc.Service,
		Kind:   components.ResourceTypeService,
	}
	if shouldEnableService(osrc, liveService) {
		actions = append(actions, Action{
			Todo:    ActionEnable,
			Parent:  comp,
			Payload: res,
		})
	}
	if !liveService.Started() {
		actions = append(actions, Action{
			Todo:    ActionStart,
			Parent:  comp,
			Payload: res,
		})
	}
	return actions, nil
}

func (m *Materia) calculateRemovedComponentResources(comp *components.Component) ([]Action, error) {
	var actions []Action
	if comp.State != components.StateNeedRemoval {
		return actions, errors.New("expected to be removed component")
	}
	resourceList := comp.Resources
	slices.Reverse(resourceList)
	for _, r := range resourceList {
		if r.Path != manifests.ComponentManifestFile {
			actions = append(actions, Action{
				Todo:    ActionRemove,
				Parent:  comp,
				Payload: r,
			})
		}
	}
	if comp.Scripted {
		actions = append(actions, Action{
			Todo:   ActionCleanup,
			Parent: comp,
		})
	}
	actions = append(actions, Action{
		Todo:    ActionRemove,
		Parent:  comp,
		Payload: components.Resource{Parent: comp.Name, Kind: components.ResourceTypeManifest, Path: manifests.ComponentManifestFile},
	})
	actions = append(actions, Action{
		Todo:    ActionRemove,
		Parent:  comp,
		Payload: components.Resource{Parent: comp.Name, Kind: components.ResourceTypeComponent, Path: comp.Name},
	})
	return actions, nil
}

func (m *Materia) processRemovedComponentServices(ctx context.Context, comp *components.Component) ([]Action, error) {
	var actions []Action
	if m.onlyResources {
		return actions, nil
	}
	for _, s := range comp.ServiceResources {
		res := components.Resource{
			Parent: comp.Name,
			Path:   s.Service,
			Kind:   components.ResourceTypeService,
		}
		liveService, err := m.Services.Get(ctx, s.Service)
		if err != nil {
			return actions, err
		}
		if liveService.Started() {
			actions = append(actions, Action{
				Todo:    ActionStop,
				Parent:  comp,
				Payload: res,
			})
		}
	}
	return actions, nil
}

func (m *Materia) diffComponent(base, other *components.Component, vars map[string]any) ([]Action, error) {
	ctx := context.TODO()
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
		currentResources[v.Path] = v
	}
	for _, v := range other.Resources {
		newResources[v.Path] = v
	}

	sortedCurrentResourceKeys := sortedKeys(currentResources)
	for _, k := range sortedCurrentResourceKeys {
		cur := currentResources[k]
		if cur.Kind == components.ResourceTypePodmanSecret {
			// validate the secret exists first
			secretsList, err := m.Containers.ListSecrets(ctx)
			if err != nil {
				return diffActions, fmt.Errorf("error listing secrets during resource validation")
			}
			if !slices.Contains(secretsList, cur.Path) {
				// secret isn't there so we treat it like the resource never existed
				delete(currentResources, k)
				continue
			}
		}
		if newRes, ok := newResources[k]; ok {
			// check for diffs and update
			log.Debug("diffing resource", "component", base.Name, "file", cur.Path)
			// TODO Refactor to not use pointers
			diffs, err := m.diffResource(&cur, &newRes, diffVars)
			if err != nil {
				return diffActions, err
			}
			if len(diffs) < 1 {
				// comparing empty files
				continue
			}
			if len(diffs) > 1 || diffs[0].Type != diffmatchpatch.DiffEqual {
				log.Debug("updating current resource", "file", cur.Path, "diffs", diffs)
				a := Action{
					Todo:    ActionUpdate,
					Parent:  other,
					Payload: newRes,
					Content: diffs,
				}

				diffActions = append(diffActions, a)
			}
		} else {
			// in current resources but not source resources, remove old
			log.Debug("removing existing resource", "file", cur.Path)
			a := Action{
				Todo:    ActionRemove,
				Parent:  base,
				Payload: cur,
			}

			diffActions = append(diffActions, a)
			if m.cleanup {
				networks, err := m.Containers.ListNetworks(ctx)
				if err != nil {
					return diffActions, err
				}
				volumes, err := m.Containers.ListVolumes(ctx)
				if err != nil {
					return diffActions, err
				}
				switch cur.Kind {
				case components.ResourceTypeNetwork:
					for _, n := range networks {
						if n.Name == cur.HostObject {
							// TODO also check that containers aren't using it
							diffActions = append(diffActions, Action{
								Todo:    ActionCleanup,
								Parent:  base,
								Payload: cur,
							})
						}
					}
				case components.ResourceTypeVolume:
					if m.cleanupVolumes {
						for _, v := range volumes {
							// TODO custome volume names
							if v.Name == cur.HostObject {
								if m.backupVolumes {
									diffActions = append(diffActions, Action{
										Todo:    ActionDump,
										Parent:  base,
										Payload: cur,
									})
								}
								diffActions = append(diffActions, Action{
									Todo:    ActionCleanup,
									Parent:  base,
									Payload: cur,
								})
							}
						}
					}
				}
			}
		}
	}
	sortedNewResourceKeys := sortedKeys(newResources)
	for _, k := range sortedNewResourceKeys {
		if _, ok := currentResources[k]; !ok {
			// if new resource is not in old resource we need to install it
			log.Debugf("Creating new resource %v", k)
			r := newResources[k]
			// do a test run just to make sure we can actually install this resource
			if r.Kind != components.ResourceTypePodmanSecret {
				newStringTempl, err := m.SourceRepo.ReadResource(r)
				if err != nil {
					return diffActions, err
				}
				resourceBody, err := m.executeResource(newStringTempl, vars)
				if err != nil {
					return diffActions, err
				}
				// update the attached object since we parsed the resource
				if r.IsQuadlet() && r.HostObject == "" {
					newName, err := parseQuadletName(r, resourceBody.String())
					if err != nil {
						return diffActions, err
					}
					r.HostObject = newName
				}

			}

			a := Action{
				Todo:    ActionInstall,
				Parent:  base,
				Payload: r,
			}
			diffActions = append(diffActions, a)
		}
	}

	return diffActions, nil
}

func parseQuadletName(r components.Resource, resourceBody string) (string, error) {
	var name string
	unitfile := parser.NewUnitFile()
	err := unitfile.Parse(resourceBody)
	if err != nil {
		return name, fmt.Errorf("error parsing container file: %w", err)
	}
	nameOption := ""
	group := ""
	switch r.Kind {
	case components.ResourceTypeContainer:
		group = "Container"
		nameOption = "ContainerName"
	case components.ResourceTypeVolume:
		group = "Volume"
		nameOption = "VolumeName"
	case components.ResourceTypeNetwork:
		group = "Network"
		nameOption = "NetworkName"
	case components.ResourceTypePod:
		group = "Pod"
		nameOption = "PodName"
	}
	if nameOption != "" {
		unitName, foundName := unitfile.Lookup(group, nameOption)
		if foundName {
			name = unitName
		} else {
			name = fmt.Sprintf("systemd-%v", filepath.Base(r.Path))
		}
	}
	return name, nil
}

func shouldEnableService(s manifests.ServiceResourceConfig, liveService *services.Service) bool {
	return !s.Disabled && s.Static && !liveService.Enabled
}

func (m *Materia) diffResource(cur, newRes *components.Resource, vars map[string]any) ([]diffmatchpatch.Diff, error) {
	dmp := diffmatchpatch.New()
	var diffs []diffmatchpatch.Diff
	if err := cur.Validate(); err != nil {
		return diffs, fmt.Errorf("self resource invalid during comparison: %w", err)
	}
	if err := newRes.Validate(); err != nil {
		return diffs, fmt.Errorf("other resource invalid during comparison: %w", err)
	}
	var curString, newString string
	var err error
	if cur.Kind != components.ResourceTypePodmanSecret {
		curString, err = m.CompRepo.ReadResource(*cur)
		if err != nil {
			return diffs, err
		}
		newStringTempl, err := m.SourceRepo.ReadResource(*newRes)
		if err != nil {
			return diffs, err
		}
		result, err := m.executeResource(newStringTempl, vars)
		if err != nil {
			return diffs, err
		}
		newString = result.String()
		if newRes.IsQuadlet() && newRes.HostObject == "" {
			newResourceName, err := parseQuadletName(*newRes, newString)
			if err != nil {
				return diffs, err
			}
			newRes.HostObject = newResourceName
		}
	} else {
		var curSecret *containers.PodmanSecret
		secretsList, err := m.Containers.ListSecrets(context.TODO())
		if err != nil {
			return diffs, err
		}
		if !slices.Contains(secretsList, cur.Path) {
			curSecret = &containers.PodmanSecret{
				Name:  cur.Path,
				Value: "",
			}
		} else {
			curSecret, err = m.Containers.GetSecret(context.TODO(), cur.Path)
			if err != nil {
				return diffs, err
			}
		}
		newSecret, ok := vars[cur.Path]
		if !ok {
			newString = ""
		} else {
			var isString bool
			newString, isString = newSecret.(string)
			if !isString {
				return diffs, errors.New("tried to use a non-string secret")
			}
		}
		curString = curSecret.Value
	}
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

func getLiveService(ctx context.Context, sm Services, serviceName string) (*services.Service, error) {
	if sm == nil {
		return nil, errors.New("need service manager")
	}
	if serviceName == "" {
		return nil, errors.New("need service name")
	}
	liveService, err := sm.Get(ctx, serviceName)
	if errors.Is(err, services.ErrServiceNotFound) {
		return &services.Service{
			Name:    serviceName,
			State:   "non-existent",
			Enabled: false,
		}, nil
	}
	return liveService, err
}
