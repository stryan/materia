package materia

import (
	"cmp"
	"fmt"
	"slices"

	"primamateria.systems/materia/internal/components"
)

type Plan struct {
	size          int
	volumes       []string
	components    []string
	servicesPhase []Action

	componentChanges map[string][]Action
}

func NewPlan(installedComps, volList []string) *Plan {
	return &Plan{
		volumes:          volList,
		components:       installedComps,
		componentChanges: make(map[string][]Action),
	}
}

func (p *Plan) Add(a Action) {
	if a.Priority == 0 {
		switch a.Target.Kind {
		case components.ResourceTypeComponent:
			switch a.Todo {
			case ActionInstall:
				a.Priority = 2
			case ActionUpdate, ActionCleanup:
				a.Priority = 3
			case ActionSetup, ActionRemove:
				a.Priority = 4
			default:
				panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Target.Path))
			}
		case components.ResourceTypeDirectory:
			switch a.Todo {
			case ActionInstall:
				a.Priority = 2
			case ActionRemove:
				a.Priority = 4
			default:
				panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Target.Path))
			}
		case components.ResourceTypeManifest:
			switch a.Todo {
			case ActionInstall, ActionRemove:
				a.Priority = 3
			case ActionUpdate:
				a.Priority = 3
			default:
				panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Target.Path))
			}
		case components.ResourceTypeFile, components.ResourceTypeContainer, components.ResourceTypePod, components.ResourceTypeKube, components.ResourceTypeNetwork, components.ResourceTypeComponentScript, components.ResourceTypeScript, components.ResourceTypePodmanSecret:
			switch a.Todo {
			case ActionInstall, ActionUpdate, ActionRemove:
				a.Priority = 3
			case ActionCleanup:
				a.Priority = 6
			case ActionDump:
				a.Priority = 6
			default:
				panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Target.Path))
			}
		case components.ResourceTypeVolume:
			switch a.Todo {
			case ActionInstall, ActionUpdate, ActionRemove:
				a.Priority = 3
			case ActionCleanup:
				a.Priority = 6
			case ActionDump:
				a.Priority = 2
			case ActionEnsure, ActionImport:
				a.Priority = 4
			default:
				panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Target.Path))
			}
		case components.ResourceTypeService:
			switch a.Todo {
			case ActionInstall, ActionUpdate, ActionRemove:
				a.Priority = 3
			case ActionRestart, ActionStart, ActionEnable, ActionDisable:
				a.Priority = 5
			case ActionStop:
				a.Priority = 1
			case ActionReload:
				a.Priority = 5
			default:
				panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Target.Path))
			}
		case components.ResourceTypeHost:
			if a.Todo == ActionReload {
				// TODO only need one reload by default
				a.Priority = 4
			} else {
				panic(fmt.Sprintf("unexpected ResourceType %v for resource %v", a.Target.Kind, a.Target))
			}
		default:
			panic(fmt.Sprintf("unexpected ResourceType %v in Action %v", a.Target.Kind, a))
		}
		if a.Target.Kind == components.ResourceTypeService && (a.Todo == ActionStart || a.Todo == ActionStop || a.Todo == ActionReload || a.Todo == ActionEnable || a.Todo == ActionDisable) {
			p.servicesPhase = append(p.servicesPhase, a)
		} else {
			p.componentChanges[a.Parent.Name] = append(p.componentChanges[a.Parent.Name], a)
		}
	} else {
		// we have a manually set priority, don't seperate out services
		p.componentChanges[a.Parent.Name] = append(p.componentChanges[a.Parent.Name], a)
	}
	p.size++
}

func (p *Plan) Append(a []Action) {
	for _, todo := range a {
		p.Add(todo)
	}
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
	sortedComps := sortedKeys(p.componentChanges)
	for _, k := range sortedComps {
		slices.SortStableFunc(p.componentChanges[k], func(a, b Action) int {
			return cmp.Compare(a.Priority, b.Priority)
		})
		steps = append(steps, p.componentChanges[k]...)
	}

	// slices.SortStableFunc(steps, func(a, b Action) int {
	// 	return cmp.Compare(a.Priority, b.Priority)
	// })
	steps = append(steps, p.servicesPhase...)

	return steps
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
