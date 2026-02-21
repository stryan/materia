package plan

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/emirpasic/gods/maps"
	"github.com/emirpasic/gods/maps/treemap"
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

type Plan struct {
	size       int
	changesMap maps.Map
}

func (p *Plan) addResourceChange(a actions.Action) {
	var changes *componentChanges
	name := a.Parent.Name
	rawChanges, ok := p.changesMap.Get(name)
	if !ok {
		changes = &componentChanges{}
	} else {
		changes = rawChanges.(*componentChanges)
	}
	changes.resourceChanges = append(changes.resourceChanges, a)
	slices.SortStableFunc(changes.resourceChanges, prioritizeActions)
	p.changesMap.Put(name, changes)
}

func (p *Plan) addServiceChange(a actions.Action) {
	var changes *componentChanges
	name := a.Parent.Name
	rawChanges, ok := p.changesMap.Get(name)
	if !ok {
		changes = &componentChanges{}
	} else {
		changes = rawChanges.(*componentChanges)
	}
	changes.serviceChanges = append(changes.serviceChanges, a)
	slices.SortStableFunc(changes.serviceChanges, prioritizeActions)
	p.changesMap.Put(name, changes)
}

func (p *Plan) getServiceChanges(name string) []actions.Action {
	rawChanges, ok := p.changesMap.Get(name)
	if !ok {
		return []actions.Action{}
	}
	changes := rawChanges.(*componentChanges)
	return changes.serviceChanges
}

func (p *Plan) getResourceChanges(name string) []actions.Action {
	rawChanges, ok := p.changesMap.Get(name)
	if !ok {
		return []actions.Action{}
	}
	changes := rawChanges.(*componentChanges)
	return changes.resourceChanges
}

func (p *Plan) listComponents() []string {
	results := make([]string, 0, p.changesMap.Size())
	for _, v := range p.changesMap.Keys() {
		results = append(results, v.(string))
	}

	return results
}

func (p *Plan) hasComponent(name string) bool {
	_, ok := p.changesMap.Get(name)
	return ok
}

func NewPlan() *Plan {
	return &Plan{
		changesMap: treemap.NewWithStringComparator(),
	}
}

func (p *Plan) Add(a actions.Action) error {
	if a.Priority == 0 {
		priority, err := getDefaultPriority(a)
		if err != nil {
			return err
		}
		a.Priority = priority

		if a.Target.Kind == components.ResourceTypeHost && a.Todo == actions.ActionReload {
			// only add an automatically prioritized Host Reload to the services phase if we don't have one at the start already
			if !p.hasComponent("root") {
				p.addServiceChange(a)
			}
		} else if a.Todo.IsServiceAction() {
			p.addServiceChange(a)
		} else {
			p.addResourceChange(a)
		}
	} else {
		// we have a manually set priority, don't seperate out services
		p.addResourceChange(a)
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

func (p *Plan) Steps() []actions.Action {
	var steps []actions.Action
	sortedComps := p.listComponents()
	for _, k := range sortedComps {
		steps = append(steps, p.getResourceChanges(k)...)
	}
	var serviceSteps []actions.Action
	for _, k := range sortedComps {
		combinedServiceActions := coalesceServices(p.getServiceChanges(k))
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
	sortedResults := make([]string, 0, len(serviceResults))
	for k := range serviceResults {
		sortedResults = append(sortedResults, k)
	}
	slices.Sort(sortedResults)

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
		components.ResourceTypeAppFile: {
			actions.ActionInstall: 3, actions.ActionUpdate: 3, actions.ActionRemove: 3,
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
