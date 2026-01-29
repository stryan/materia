package plan

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"

	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/pkg/components"
)

type componentChanges struct {
	resourceChanges []actions.Action
	serviceChanges  []actions.Action
}

func prioritizeActions(a, b actions.Action) int {
	return cmp.Compare(a.Priority, b.Priority)
}

func (c *componentChanges) addResourceChange(a actions.Action) {
	c.resourceChanges = append(c.resourceChanges, a)
	slices.SortStableFunc(c.resourceChanges, prioritizeActions)
}

func (c *componentChanges) addServiceChange(a actions.Action) {
	c.serviceChanges = append(c.serviceChanges, a)
	slices.SortStableFunc(c.serviceChanges, prioritizeActions)
}

type Plan struct {
	size       int
	volumes    []string
	components []string

	changes map[string]*componentChanges
}

func NewPlan(installedComps, volList []string) *Plan {
	return &Plan{
		volumes:    volList,
		components: installedComps,
		changes:    make(map[string]*componentChanges),
	}
}

func (p *Plan) Add(a actions.Action) error {
	if a.Priority == 0 {
		priority, err := getDefaultPriority(a)
		if err != nil {
			return err
		}
		a.Priority = priority

		changes, ok := p.changes[a.Parent.Name]
		if !ok {
			changes = &componentChanges{}
		}
		if a.Target.Kind == components.ResourceTypeHost && a.Todo == actions.ActionReload {
			// only add an automatically prioritized Host Reload to the services phase if we don't have one at the start already
			if _, ok := p.changes["root"]; !ok {
				changes.addServiceChange(a)
			}
		} else if a.Todo.IsServiceAction() {
			changes.addServiceChange(a)
		} else {
			changes.addResourceChange(a)
		}
		p.changes[a.Parent.Name] = changes
	} else {
		// we have a manually set priority, don't seperate out services
		changes, ok := p.changes[a.Parent.Name]
		if !ok {
			changes = &componentChanges{}
		}
		changes.addResourceChange(a)
		p.changes[a.Parent.Name] = changes
	}
	p.size++
	return nil
}

func (p *Plan) Append(a []actions.Action) error {
	for _, todo := range a {
		err := p.Add(todo)
		if err != nil {
			return fmt.Errorf("unable to append actions to plan: %w", err)
		}
	}
	return nil
}

func (p *Plan) Empty() bool {
	return p.size == 0
}

func (p *Plan) Size() int {
	return p.size
}

func (p *Plan) Validate() error {
	steps := p.Steps()
	componentList := p.components
	needReload := false
	reload := false
	deletedVoles := []string{}
	currentStep := 1
	maxSteps := len(steps)
	for _, a := range steps {
		if (a.Target.Kind == components.ResourceTypeService || a.Target.IsQuadlet()) && a.Todo == actions.ActionInstall {
			needReload = true
		}
		if a.Target.Kind == components.ResourceTypeCombined {
			return fmt.Errorf("%v/%v invalid plan: tried to act on a combined resource: %v", currentStep, maxSteps, a.Target.Path)
		}
		if a.Todo == actions.ActionReload && a.Target.Path == "" {
			reload = true
		}
		if a.Target.IsQuadlet() && a.Target.HostObject == "" {
			return fmt.Errorf("%v/%v: tried to operate on a quadlet without a backing podman object: %v", currentStep, maxSteps, a.Target)
		}
		if a.Todo == actions.ActionRemove && a.Target.Kind == components.ResourceTypeVolume {
			deletedVoles = append(deletedVoles, a.Target.Path)
		}
		if a.Todo == actions.ActionDump && a.Target.Kind == components.ResourceTypeVolume {
			if slices.Contains(deletedVoles, a.Target.Path) {
				return fmt.Errorf("%v/%v: invalid plan: deleted volume %v before dumping", currentStep, maxSteps, a.Target.Path)
			}
		}

		if a.Todo == actions.ActionInstall {
			if a.Target.Kind == components.ResourceTypeComponent {
				componentList = append(componentList, a.Parent.Name)
			} else {
				if !slices.Contains(componentList, a.Parent.Name) {
					return fmt.Errorf("%v/%v: invalid plan: installed resource %v before parent component %v", currentStep, maxSteps, a.Target, a.Parent.Name)
				}
			}
		}
		currentStep++
	}
	if needReload && !reload {
		return fmt.Errorf("invalid plan: %v/%v: systemd units added without a daemon-reload", currentStep, maxSteps) // yeah yeah this is always at the end
	}

	return nil
}

func (p *Plan) Steps() []actions.Action {
	var steps []actions.Action
	sortedComps := sortedKeys(p.changes)
	for _, k := range sortedComps {
		resourceSteps := p.changes[k].resourceChanges
		steps = append(steps, resourceSteps...)
	}
	var serviceSteps []actions.Action
	for _, k := range sortedComps {
		serviceActions := p.changes[k].serviceChanges
		combinedServiceActions := coalesceServices(serviceActions)
		serviceSteps = append(serviceSteps, combinedServiceActions...)
	}
	slices.SortStableFunc(serviceSteps, prioritizeActions)
	steps = append(steps, serviceSteps...)
	slices.SortStableFunc(steps, prioritizeActions)

	return steps
}

func coalesceServices(changes []actions.Action) []actions.Action {
	var results []actions.Action
	serviceResults := make(map[string]int)
	serviceActions := make(map[string]actions.Action)
	for _, a := range changes {
		if _, ok := serviceResults[a.Target.Path]; !ok {
			serviceResults[a.Target.Path] = 0
		}
		endState := serviceResults[a.Target.Path]
		if a.Todo == actions.ActionEnable || a.Todo == actions.ActionDisable {
			// don't need to coalesce enabling/disabling services
			// if someone wants to enable and disable a service in the same plan, who are we to judge
			results = append(results, a)
			continue
		}
		if a.Todo == actions.ActionReload && endState < 1 {
			endState = 1
			serviceActions[a.Target.Path] = a
		}
		if a.Todo == actions.ActionStart && endState < 2 {
			endState = 2
			serviceActions[a.Target.Path] = a
		}
		if a.Todo == actions.ActionRestart && endState < 3 {
			endState = 3
			serviceActions[a.Target.Path] = a
		}
		if a.Todo == actions.ActionStop && endState < 4 {
			endState = 4
			serviceActions[a.Target.Path] = a
		}
		serviceResults[a.Target.Path] = endState
	}
	sortedResults := sortedKeys(serviceResults)
	for _, k := range sortedResults {
		results = append(results, serviceActions[k])
	}
	return results
}

func (p *Plan) Pretty() string {
	if p.Empty() {
		return "Nothing to do"
	}
	var result string
	steps := p.Steps()
	result += "Plan: \n"
	for i, a := range steps {
		result += fmt.Sprintf("%v. %v\n", i+1, a.Pretty())
	}
	return result
}

func (p *Plan) ToJson() ([]byte, error) {
	actions := p.Steps()
	return json.Marshal(actions)
}

func (p *Plan) PrettyLines() []string {
	if p.Empty() {
		return []string{""}
	}
	var result []string
	steps := p.Steps()
	for i, a := range steps {
		result = append(result, fmt.Sprintf("%v. %v", i+1, a.Pretty()))
	}
	return result
}

func getDefaultPriority(a actions.Action) (int, error) {
	priorityMap := map[components.ResourceType]map[actions.ActionType]int{
		components.ResourceTypeComponent: {
			actions.ActionInstall: 2, actions.ActionUpdate: 3, actions.ActionCleanup: 2,
			actions.ActionSetup: 5, actions.ActionRemove: 4,
		},
		components.ResourceTypeDirectory: {
			actions.ActionInstall: 2, actions.ActionRemove: 4,
		},
		components.ResourceTypeManifest: {
			actions.ActionInstall: 3, actions.ActionUpdate: 3, actions.ActionRemove: 3,
		},
		components.ResourceTypeFile: {
			actions.ActionInstall: 3, actions.ActionUpdate: 3, actions.ActionRemove: 3,
			actions.ActionCleanup: 7, actions.ActionDump: 2,
		},
		components.ResourceTypeContainer: {
			actions.ActionInstall: 3, actions.ActionUpdate: 3, actions.ActionRemove: 3,
			actions.ActionCleanup: 7, actions.ActionDump: 2,
			actions.ActionStart: 6, actions.ActionStop: 1, actions.ActionRestart: 6, actions.ActionReload: 6,
		},
		components.ResourceTypePod: {
			actions.ActionInstall: 3, actions.ActionUpdate: 3, actions.ActionRemove: 3,
			actions.ActionCleanup: 7, actions.ActionDump: 2,
			actions.ActionStart: 6, actions.ActionStop: 1, actions.ActionRestart: 6, actions.ActionReload: 6,
		},
		components.ResourceTypeKube: {
			actions.ActionInstall: 3, actions.ActionUpdate: 3, actions.ActionRemove: 3,
			actions.ActionCleanup: 7, actions.ActionDump: 2,
			actions.ActionStart: 6, actions.ActionStop: 1, actions.ActionRestart: 6, actions.ActionReload: 6,
		},
		components.ResourceTypeNetwork: {
			actions.ActionInstall: 3, actions.ActionUpdate: 3, actions.ActionRemove: 3,
			actions.ActionCleanup: 7, actions.ActionDump: 2, actions.ActionEnsure: 5,
			actions.ActionStart: 6, actions.ActionStop: 1, actions.ActionRestart: 6, actions.ActionReload: 6,
		},
		components.ResourceTypeBuild: {
			actions.ActionInstall: 3, actions.ActionUpdate: 3, actions.ActionRemove: 3,
			actions.ActionCleanup: 7, actions.ActionDump: 2,
			actions.ActionStart: 6, actions.ActionStop: 1, actions.ActionRestart: 6, actions.ActionReload: 6,
		},
		components.ResourceTypeImage: {
			actions.ActionInstall: 3, actions.ActionUpdate: 3, actions.ActionRemove: 3,
			actions.ActionCleanup: 7, actions.ActionDump: 2,
			actions.ActionStart: 6, actions.ActionStop: 1, actions.ActionRestart: 6, actions.ActionReload: 6,
		},
		components.ResourceTypeScript: {
			actions.ActionInstall: 3, actions.ActionUpdate: 3, actions.ActionRemove: 3,
			actions.ActionCleanup: 7, actions.ActionDump: 2,
		},
		components.ResourceTypePodmanSecret: {
			actions.ActionInstall: 3, actions.ActionUpdate: 3, actions.ActionRemove: 3,
			actions.ActionCleanup: 7, actions.ActionDump: 2,
		},
		components.ResourceTypeVolume: {
			actions.ActionInstall: 3, actions.ActionUpdate: 3, actions.ActionRemove: 3,
			actions.ActionCleanup: 7, actions.ActionDump: 2, actions.ActionEnsure: 5, actions.ActionImport: 4,
			actions.ActionStart: 6, actions.ActionStop: 1, actions.ActionRestart: 6, actions.ActionReload: 6,
		},
		components.ResourceTypeService: {
			actions.ActionInstall: 3, actions.ActionUpdate: 3, actions.ActionRemove: 3,
			actions.ActionRestart: 6, actions.ActionStart: 6, actions.ActionEnable: 6,
			actions.ActionDisable: 6, actions.ActionStop: 1, actions.ActionReload: 6,
		},
		components.ResourceTypeHost: {
			actions.ActionReload: 4,
		},
	}

	if priorities, ok := priorityMap[a.Target.Kind]; ok {
		if priority, ok := priorities[a.Todo]; ok {
			return priority, nil
		}
	}
	return -1, fmt.Errorf("invalid action type %v for resource type %v", a.Todo, a.Target.Kind)
}

func sortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	keys := make([]K, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	slices.Sort(keys)
	return keys
}
