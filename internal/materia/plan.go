package materia

import (
	"errors"
	"fmt"
	"slices"

	"primamateria.systems/materia/internal/components"
)

type Plan struct {
	size       int
	volumes    []string
	components []string

	combatPhase []Action
	secondMain  []Action
	endStep     []Action

	resourceChanges  map[string][]Action
	structureChanges map[string][]Action
}

func NewPlan(installedComps, volList []string) *Plan {
	return &Plan{
		volumes:          volList,
		components:       installedComps,
		resourceChanges:  make(map[string][]Action),
		structureChanges: make(map[string][]Action),
	}
}

func (p *Plan) Add(a Action) {
	switch a.Payload.Kind {
	case components.ResourceTypeComponent:
		switch a.Todo {
		case ActionInstall, ActionRemove:
			p.structureChanges[a.Parent.Name] = append(p.structureChanges[a.Parent.Name], a)
		case ActionUpdate, ActionCleanup:
			p.resourceChanges[a.Parent.Name] = append(p.resourceChanges[a.Parent.Name], a)
		case ActionSetup:
			p.secondMain = append(p.secondMain, a)
		default:
			panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Payload.Name))
		}
	case components.ResourceTypeDirectory:
		switch a.Todo {
		case ActionInstall, ActionRemove:
			p.structureChanges[a.Parent.Name] = append(p.structureChanges[a.Parent.Name], a)
		default:
			panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Payload.Name))
		}
	case components.ResourceTypeFile, components.ResourceTypeContainer, components.ResourceTypeVolume, components.ResourceTypePod, components.ResourceTypeKube, components.ResourceTypeNetwork, components.ResourceTypeComponentScript, components.ResourceTypeScript, components.ResourceTypePodmanSecret, components.ResourceTypeManifest:
		switch a.Todo {
		case ActionInstall, ActionUpdate, ActionRemove:
			p.resourceChanges[a.Parent.Name] = append(p.resourceChanges[a.Parent.Name], a)
		default:
			panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Payload.Name))
		}
	case components.ResourceTypeVolumeFile:
		switch a.Todo {
		case ActionInstall:
			p.secondMain = append(p.secondMain, a)
			vcr, ok := a.Parent.VolumeResources[a.Payload.Name]
			if !ok {
				return
			}
			p.volumes = append(p.volumes, vcr.Volume)
		case ActionRemove, ActionUpdate:
			p.secondMain = append(p.secondMain, a)
		default:
			panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Payload.Name))
		}
	case components.ResourceTypeService:
		switch a.Todo {
		case ActionInstall, ActionUpdate, ActionRemove:
			p.resourceChanges[a.Parent.Name] = append(p.resourceChanges[a.Parent.Name], a)
		case ActionRestart, ActionStart, ActionStop, ActionEnable, ActionDisable:
			if slices.ContainsFunc(p.endStep, func(modification Action) bool {
				return (modification.Payload.Name == a.Payload.Name && modification.Todo == a.Todo)
			}) {
				return
			}
			p.endStep = append(p.endStep, a)

		case ActionReload:
			if slices.ContainsFunc(p.endStep, func(modification Action) bool {
				return (modification.Payload.Name == a.Payload.Name && modification.Todo == a.Todo)
			}) {
				return
			}
			p.endStep = append(p.endStep, a)
		default:
			panic(fmt.Sprintf("unexpected Action %v for Resource %v", a.Todo, a.Payload.Name))
		}
	case components.ResourceTypeHost:
		if a.Todo == ActionReload {
			if len(p.combatPhase) == 0 || (p.combatPhase[0].Todo != ActionReload && p.combatPhase[0].Parent.Name != "") {
				// only need to reload once but we do need to do it before any other service actions
				p.combatPhase = slices.Insert(p.combatPhase, 0, a)
			}
		} else {
			panic(fmt.Sprintf("unexpected ResourceType %v for resource %v", a.Payload.Kind, a.Payload))
		}
	default:
		panic(fmt.Sprintf("unexpected ResourceType %v for resource %v", a.Payload.Kind, a.Payload))
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
	for _, a := range steps {
		if (a.Payload.Kind == components.ResourceTypeService || a.Payload.IsQuadlet()) && a.Todo == ActionInstall {
			needReload = true
		}
		if a.Todo == ActionReload && a.Payload.Name == "" {
			reload = true
		}
		if a.Payload.Kind == components.ResourceTypeVolumeFile {
			vcr, ok := a.Parent.VolumeResources[a.Payload.Name]
			if !ok {
				return fmt.Errorf("invalid plan: no volume resource for %v", a.Payload)
			}
			if slices.Contains(p.volumes, vcr.Volume) {
				continue
			}
			return fmt.Errorf("invalid plan: no volume for resource %v", a.Payload)
		}
		if a.Todo == ActionInstall {
			if a.Payload.Kind == components.ResourceTypeComponent {
				componentList = append(componentList, a.Parent.Name)
			} else {
				if !slices.Contains(componentList, a.Parent.Name) {
					return fmt.Errorf("invalid plan: installed resource %v before parent component %v", a.Payload, a.Parent.Name)
				}
			}
		}
	}
	if needReload && !reload {
		return errors.New("invalid plan: systemd units added without a daemon-reload")
	}

	return nil
}

func (p *Plan) Steps() []Action {
	var mainPhase []Action
	keys := make([]string, 0, len(p.resourceChanges))
	for k := range p.resourceChanges {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		componentActions := []Action{}
		beginningStep := []Action{}
		endstep := []Action{}
		for _, sc := range p.structureChanges[k] {
			if sc.Todo == ActionInstall && (sc.Payload.Kind == components.ResourceTypeComponent || sc.Payload.Kind == components.ResourceTypeDirectory) {
				beginningStep = append(beginningStep, sc)
			} else {
				endstep = append(endstep, sc)
			}
		}
		componentActions = append(componentActions, beginningStep...)
		componentActions = append(componentActions, p.resourceChanges[k]...)
		componentActions = append(componentActions, endstep...)
		mainPhase = append(mainPhase, componentActions...)
	}

	return slices.Concat(mainPhase, p.combatPhase, p.secondMain, p.endStep)
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
