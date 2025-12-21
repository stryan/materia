package materia

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"

	"primamateria.systems/materia/internal/components"
)

type componentChanges struct {
	resourceChanges []Action
	serviceChanges  []Action
}

func prioritizeActions(a, b Action) int {
	return cmp.Compare(a.Priority, b.Priority)
}

func (c *componentChanges) addResourceChange(a Action) {
	c.resourceChanges = append(c.resourceChanges, a)
	slices.SortStableFunc(c.resourceChanges, prioritizeActions)
}

func (c *componentChanges) addServiceChange(a Action) {
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

func (p *Plan) Add(a Action) error {
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
		if a.Target.Kind == components.ResourceTypeHost && a.Todo == ActionReload {
			// only add an automatically prioritized Host Reload to the services phase if we don't have one at the start already
			if _, ok := p.changes[rootComponent.Name]; !ok {
				changes.addServiceChange(a)
			}
		} else if a.Todo == ActionStart || a.Todo == ActionStop || a.Todo == ActionReload || a.Todo == ActionEnable || a.Todo == ActionDisable || a.Todo == ActionRestart {
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

func (p *Plan) Append(a []Action) error {
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
		if (a.Target.Kind == components.ResourceTypeService || a.Target.IsQuadlet()) && a.Todo == ActionInstall {
			needReload = true
		}
		if a.Todo == ActionReload && a.Target.Path == "" {
			reload = true
		}
		if a.Target.IsQuadlet() && a.Target.HostObject == "" {
			return fmt.Errorf("%v/%v: tried to operate on a quadlet without a backing podman object: %v", currentStep, maxSteps, a.Target)
		}
		if a.Todo == ActionRemove && a.Target.Kind == components.ResourceTypeVolume {
			deletedVoles = append(deletedVoles, a.Target.Path)
		}
		if a.Todo == ActionDump && a.Target.Kind == components.ResourceTypeVolume {
			if slices.Contains(deletedVoles, a.Target.Path) {
				return fmt.Errorf("%v/%v: invalid plan: deleted volume %v before dumping", currentStep, maxSteps, a.Target.Path)
			}
		}

		if a.Todo == ActionInstall {
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

func (p *Plan) Steps() []Action {
	var steps []Action
	sortedComps := sortedKeys(p.changes)
	for _, k := range sortedComps {
		resourceSteps := p.changes[k].resourceChanges
		steps = append(steps, resourceSteps...)
	}
	var serviceSteps []Action
	for _, k := range sortedComps {
		servActions := p.changes[k].serviceChanges
		combinedServiceActions := coalesceServices(servActions)
		serviceSteps = append(serviceSteps, combinedServiceActions...)
	}
	slices.SortStableFunc(serviceSteps, prioritizeActions)
	steps = append(steps, serviceSteps...)

	return steps
}

func coalesceServices(changes []Action) []Action {
	var results []Action
	serviceResults := make(map[string]int)
	serviceActions := make(map[string]Action)
	for _, a := range changes {
		if _, ok := serviceResults[a.Target.Path]; !ok {
			serviceResults[a.Target.Path] = 0
		}
		endState := serviceResults[a.Target.Path]
		if a.Todo == ActionEnable || a.Todo == ActionDisable {
			// don't need to coalesce enabling/disabling services
			// if someone wants to enable and disable a service in the same plan, who are we to judge
			results = append(results, a)
			continue
		}
		if a.Todo == ActionReload && endState < 1 {
			endState = 1
			serviceActions[a.Target.Path] = a
		}
		if a.Todo == ActionStart && endState < 2 {
			endState = 2
			serviceActions[a.Target.Path] = a
		}
		if a.Todo == ActionRestart && endState < 3 {
			endState = 3
			serviceActions[a.Target.Path] = a
		}
		if a.Todo == ActionStop && endState < 4 {
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

func getDefaultPriority(a Action) (int, error) {
	priorityMap := map[components.ResourceType]map[ActionType]int{
		components.ResourceTypeComponent: {
			ActionInstall: 2, ActionUpdate: 3, ActionCleanup: 3,
			ActionSetup: 4, ActionRemove: 4,
		},
		components.ResourceTypeDirectory: {
			ActionInstall: 2, ActionRemove: 4,
		},
		components.ResourceTypeManifest: {
			ActionInstall: 3, ActionUpdate: 3, ActionRemove: 3,
		},
		components.ResourceTypeFile: {
			ActionInstall: 3, ActionUpdate: 3, ActionRemove: 3,
			ActionCleanup: 6, ActionDump: 2,
		},
		components.ResourceTypeContainer: {
			ActionInstall: 3, ActionUpdate: 3, ActionRemove: 3,
			ActionCleanup: 6, ActionDump: 2,
		},
		components.ResourceTypePod: {
			ActionInstall: 3, ActionUpdate: 3, ActionRemove: 3,
			ActionCleanup: 6, ActionDump: 2,
		},
		components.ResourceTypeKube: {
			ActionInstall: 3, ActionUpdate: 3, ActionRemove: 3,
			ActionCleanup: 6, ActionDump: 2,
		},
		components.ResourceTypeNetwork: {
			ActionInstall: 3, ActionUpdate: 3, ActionRemove: 3,
			ActionCleanup: 6, ActionDump: 2,
		},
		components.ResourceTypeBuild: {
			ActionInstall: 3, ActionUpdate: 3, ActionRemove: 3,
			ActionCleanup: 6, ActionDump: 2,
		},
		components.ResourceTypeImage: {
			ActionInstall: 3, ActionUpdate: 3, ActionRemove: 3,
			ActionCleanup: 6, ActionDump: 2,
		},
		components.ResourceTypeComponentScript: {
			ActionInstall: 3, ActionUpdate: 3, ActionRemove: 3,
			ActionCleanup: 6, ActionDump: 2,
		},
		components.ResourceTypeScript: {
			ActionInstall: 3, ActionUpdate: 3, ActionRemove: 3,
			ActionCleanup: 6, ActionDump: 2,
		},
		components.ResourceTypePodmanSecret: {
			ActionInstall: 3, ActionUpdate: 3, ActionRemove: 3,
			ActionCleanup: 6, ActionDump: 2,
		},
		components.ResourceTypeVolume: {
			ActionInstall: 3, ActionUpdate: 3, ActionRemove: 3,
			ActionCleanup: 6, ActionDump: 2, ActionEnsure: 4, ActionImport: 4,
		},
		components.ResourceTypeService: {
			ActionInstall: 3, ActionUpdate: 3, ActionRemove: 3,
			ActionRestart: 5, ActionStart: 5, ActionEnable: 5,
			ActionDisable: 5, ActionStop: 1, ActionReload: 5,
		},
		components.ResourceTypeHost: {
			ActionReload: 4,
		},
	}

	if priorities, ok := priorityMap[a.Target.Kind]; ok {
		if priority, ok := priorities[a.Todo]; ok {
			return priority, nil
		}
	}
	return -1, fmt.Errorf("invalid action type %v for resource type %v", a.Todo, a.Target.Kind)
}
