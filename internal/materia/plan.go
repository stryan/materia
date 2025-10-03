package materia

import (
	"fmt"
	"slices"

	"primamateria.systems/materia/internal/components"
)

type Plan struct {
	size                int
	volumes             []string
	components          []string
	serviceRemovalPhase []Action
	combatPhase         []Action
	secondMain          []Action
	servicesPhase       []Action

	resourceChanges  map[string][]Action
	structureChanges map[string][]Action
	cleanupChanges   map[string][]Action
}

func NewPlan(installedComps, volList []string) *Plan {
	return &Plan{
		volumes:          volList,
		components:       installedComps,
		resourceChanges:  make(map[string][]Action),
		structureChanges: make(map[string][]Action),
		cleanupChanges:   make(map[string][]Action),
	}
}

func (p *Plan) Add(a Action) {
	switch a.Target.Kind {
	case components.ResourceTypeComponent:
		switch a.Todo {
		case ActionInstall, ActionRemove:
			p.structureChanges[a.Parent.Name] = append(p.structureChanges[a.Parent.Name], a)
		case ActionUpdate, ActionCleanup:
			p.resourceChanges[a.Parent.Name] = append(p.resourceChanges[a.Parent.Name], a)
		case ActionSetup:
			p.secondMain = append(p.secondMain, a)
		default:
			panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Target.Path))
		}
	case components.ResourceTypeDirectory:
		switch a.Todo {
		case ActionInstall, ActionRemove:
			p.structureChanges[a.Parent.Name] = append(p.structureChanges[a.Parent.Name], a)
		default:
			panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Target.Path))
		}
	case components.ResourceTypeManifest:
		switch a.Todo {
		case ActionInstall, ActionRemove:
			p.structureChanges[a.Parent.Name] = append(p.structureChanges[a.Parent.Name], a)
		case ActionUpdate:
			p.resourceChanges[a.Parent.Name] = append(p.resourceChanges[a.Parent.Name], a)
		default:
			panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Target.Path))
		}
	case components.ResourceTypeFile, components.ResourceTypeContainer, components.ResourceTypeVolume, components.ResourceTypePod, components.ResourceTypeKube, components.ResourceTypeNetwork, components.ResourceTypeComponentScript, components.ResourceTypeScript, components.ResourceTypePodmanSecret:
		switch a.Todo {
		case ActionInstall, ActionUpdate, ActionRemove:
			p.resourceChanges[a.Parent.Name] = append(p.resourceChanges[a.Parent.Name], a)
		case ActionCleanup:
			p.cleanupChanges[a.Parent.Name] = append(p.cleanupChanges[a.Parent.Name], a)
		case ActionDump:
			p.cleanupChanges[a.Parent.Name] = append(p.cleanupChanges[a.Parent.Name], a)
		default:
			panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Target.Path))
		}
	case components.ResourceTypeService:
		switch a.Todo {
		case ActionInstall, ActionUpdate, ActionRemove:
			p.resourceChanges[a.Parent.Name] = append(p.resourceChanges[a.Parent.Name], a)
		case ActionRestart, ActionStart, ActionEnable, ActionDisable:
			if slices.ContainsFunc(p.servicesPhase, func(modification Action) bool {
				return (modification.Target.Path == a.Target.Path && modification.Todo == a.Todo)
			}) {
				return
			}
			p.servicesPhase = append(p.servicesPhase, a)
		case ActionStop:
			if slices.ContainsFunc(p.servicesPhase, func(modification Action) bool {
				return (modification.Target.Path == a.Target.Path && modification.Todo == a.Todo)
			}) {
				return
			}
			p.serviceRemovalPhase = append(p.serviceRemovalPhase, a)
		case ActionReload:
			if slices.ContainsFunc(p.servicesPhase, func(modification Action) bool {
				return (modification.Target.Path == a.Target.Path && modification.Todo == a.Todo)
			}) {
				return
			}
			p.servicesPhase = append(p.servicesPhase, a)
		default:
			panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Target.Path))
		}
	case components.ResourceTypeHost:
		if a.Todo == ActionReload {
			if len(p.combatPhase) == 0 || (p.combatPhase[0].Todo != ActionReload && p.combatPhase[0].Parent.Name != "") {
				// only need to reload once but we do need to do it before any other service actions
				p.combatPhase = slices.Insert(p.combatPhase, 0, a)
			}
		} else {
			panic(fmt.Sprintf("unexpected ResourceType %v for resource %v", a.Target.Kind, a.Target))
		}
	default:
		panic(fmt.Sprintf("unexpected ResourceType %v in Action %v", a.Target.Kind, a))
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
			if !slices.Contains(deletedVoles, a.Target.Path) {
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
	var mainPhase []Action
	var cleanupPhase []Action
	keys := sortedKeys(p.resourceChanges)
	for _, k := range keys {
		componentActions := []Action{}
		beginningStep := []Action{}
		endstep := []Action{}
		for _, sc := range p.structureChanges[k] {
			if sc.Todo == ActionInstall && (sc.Target.Kind == components.ResourceTypeComponent || sc.Target.Kind == components.ResourceTypeDirectory) {
				beginningStep = append(beginningStep, sc)
			} else {
				endstep = append(endstep, sc)
			}
		}
		componentActions = append(componentActions, beginningStep...)
		componentActions = append(componentActions, p.resourceChanges[k]...)
		componentActions = append(componentActions, endstep...)
		mainPhase = append(mainPhase, componentActions...)
		cleanupPhase = append(cleanupPhase, p.cleanupChanges[k]...)

	}

	return slices.Concat(p.serviceRemovalPhase, mainPhase, p.combatPhase, p.secondMain, p.servicesPhase, cleanupPhase)
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
