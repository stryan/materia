package materia

import (
	"errors"
	"fmt"
	"slices"
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
	switch a.Todo {
	case ActionInstallDirectory, ActionInstallComponent, ActionRemoveComponent, ActionRemoveDirectory:
		p.structureChanges[a.Parent.Name] = append(p.structureChanges[a.Parent.Name], a)
	case ActionInstallFile, ActionInstallQuadlet, ActionInstallScript, ActionInstallService, ActionInstallComponentScript, ActionUpdateFile, ActionUpdateQuadlet, ActionUpdateScript, ActionUpdateService, ActionUpdateComponentScript, ActionRemoveFile, ActionRemoveQuadlet, ActionRemoveScript, ActionRemoveService, ActionRemoveComponentScript, ActionUpdateComponent, ActionCleanupComponent, ActionInstallPodmanSecret, ActionUpdatePodmanSecret, ActionRemovePodmanSecret:
		p.resourceChanges[a.Parent.Name] = append(p.resourceChanges[a.Parent.Name], a)
	case ActionInstallVolumeFile:
		p.secondMain = append(p.secondMain, a)
		vcr, ok := a.Parent.VolumeResources[a.Payload.Name]
		if !ok {
			return
		}
		p.volumes = append(p.volumes, vcr.Volume)
	case ActionRemoveVolumeFile, ActionUpdateVolumeFile:
		p.secondMain = append(p.secondMain, a)
	case ActionReloadUnits:
		if len(p.combatPhase) == 0 || p.combatPhase[0].Todo != ActionReloadUnits {
			// only need to reload once but we do need to do it before any other service actions
			p.combatPhase = slices.Insert(p.combatPhase, 0, a)
		}
	case ActionEnsureVolume:
		// only ensure each volume once
		if slices.ContainsFunc(p.combatPhase, func(combat Action) bool {
			return combat.Payload.Name == a.Payload.Name
		}) {
			return
		}
		p.combatPhase = append(p.combatPhase, a)
	case ActionRestartService, ActionStartService, ActionStopService, ActionEnableService, ActionDisableService, ActionReloadService:
		// modify each service only once per action
		if slices.ContainsFunc(p.endStep, func(modification Action) bool {
			return (modification.Payload.Name == a.Payload.Name && modification.Todo == a.Todo)
		}) {
			return
		}
		p.endStep = append(p.endStep, a)
	case ActionSetupComponent:
		p.secondMain = append(p.secondMain, a)
	default:
		panic(fmt.Sprintf("unexpected materia.ActionType: %v : %v", a.Todo, a))
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
	components := p.components
	needReload := false
	reload := false
	for _, a := range steps {
		if a.Todo == ActionInstallService || a.Todo == ActionInstallQuadlet {
			needReload = true
		}
		if a.Todo == ActionReloadUnits {
			reload = true
		}
		if a.Todo == ActionInstallVolumeFile || a.Todo == ActionUpdateVolumeFile || a.Todo == ActionRemoveVolumeFile {
			vcr, ok := a.Parent.VolumeResources[a.Payload.Name]
			if !ok {
				return fmt.Errorf("invalid plan: no volume resource for %v", a.Payload)
			}
			if slices.Contains(p.volumes, vcr.Volume) {
				continue
			}
			return fmt.Errorf("invalid plan: no volume for resource %v", a.Payload)
		}
		if a.Category() == ActionCategoryInstall {
			if a.Todo == ActionInstallComponent {
				components = append(components, a.Parent.Name)
			} else {
				if !slices.Contains(components, a.Parent.Name) {
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
			if sc.Todo == ActionInstallComponent || sc.Todo == ActionInstallDirectory {
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
