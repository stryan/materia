package materia

import (
	"fmt"
	"slices"
)

type Plan struct {
	volumes []string

	mainPhase   []Action
	combatPhase []Action
	secondMain  []Action
	endStep     []Action
}

func NewPlan(facts *Facts) *Plan {
	p := &Plan{}
	for _, v := range facts.Volumes {
		p.volumes = append(p.volumes, v.Name)
	}
	return p
}

func (p *Plan) Add(a Action) {
	switch a.Todo {
	case ActionCleanupComponent:
		p.mainPhase = append(p.mainPhase, a)
	case ActionInstallComponent, ActionInstallResource, ActionRemoveComponent, ActionRemoveResource, ActionUpdateResource:
		p.mainPhase = append(p.mainPhase, a)
	case ActionInstallVolumeResource:
		p.secondMain = append(p.secondMain, a)
		vcr, ok := a.Parent.VolumeResources[a.Payload.Name]
		if !ok {
			return
		}
		p.volumes = append(p.volumes, vcr.Volume)
	case ActionRemoveVolumeResource, ActionUpdateVolumeResource:
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
	case ActionRestartService, ActionStartService, ActionStopService:
		p.endStep = append(p.endStep, a)
	case ActionSetupComponent:
		p.secondMain = append(p.secondMain, a)
	default:
		panic(fmt.Sprintf("unexpected materia.ActionType: %#v", a.Todo))
	}
}

func (p *Plan) Append(a []Action) {
	for _, todo := range a {
		p.Add(todo)
	}
}

func (p *Plan) Empty() bool {
	return len(p.mainPhase) == 0 && len(p.combatPhase) == 0 && len(p.secondMain) == 0 && len(p.endStep) == 0
}

func (p *Plan) Validate() error {
	steps := slices.Concat(p.mainPhase, p.combatPhase, p.secondMain, p.endStep)
	for _, a := range steps {
		if a.Todo == ActionInstallVolumeResource || a.Todo == ActionUpdateVolumeResource || a.Todo == ActionRemoveVolumeResource {
			vcr, ok := a.Parent.VolumeResources[a.Payload.Name]
			if !ok {
				return fmt.Errorf("invalid plan: no volume resource for %v", a.Payload)
			}
			if slices.Contains(p.volumes, vcr.Volume) {
				continue
			}
			return fmt.Errorf("invalid plan: no volume for resource %v", a.Payload)
		}
	}

	return nil
}

func (p *Plan) Steps() []Action {
	return slices.Concat(p.mainPhase, p.combatPhase, p.secondMain, p.endStep)
}
