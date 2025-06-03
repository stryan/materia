package materia

import (
	"errors"
	"fmt"
	"slices"

	fprov "git.saintnet.tech/stryan/materia/internal/facts"
)

type Plan struct {
	volumes    []string
	components []string

	mainPhase   []Action
	combatPhase []Action
	secondMain  []Action
	endStep     []Action
}

func NewPlan(facts fprov.FactsProvider) *Plan {
	p := &Plan{}
	for _, v := range facts.GetVolumes() {
		p.volumes = append(p.volumes, v.Name)
	}
	p.components = append(p.components, facts.GetInstalledComponents()...)
	return p
}

func (p *Plan) Add(a Action) {
	switch a.Todo {
	case ActionCleanupComponent:
		p.mainPhase = append(p.mainPhase, a)
	case ActionInstallComponent, ActionRemoveComponent, ActionInstallFile, ActionInstallQuadlet, ActionInstallScript, ActionInstallService, ActionInstallComponentScript, ActionUpdateFile, ActionUpdateQuadlet, ActionUpdateScript, ActionUpdateService, ActionUpdateComponentScript, ActionRemoveFile, ActionRemoveQuadlet, ActionRemoveScript, ActionRemoveService, ActionRemoveComponentScript:
		p.mainPhase = append(p.mainPhase, a)
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
		panic(fmt.Sprintf("unexpected materia.ActionType: %v", a.Todo))
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
	return slices.Concat(p.mainPhase, p.combatPhase, p.secondMain, p.endStep)
}

func (p *Plan) Pretty() string {
	if p.Empty() {
		return "Nothing to do"
	}
	var result string
	steps := slices.Concat(p.mainPhase, p.combatPhase, p.secondMain, p.endStep)
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
	steps := slices.Concat(p.mainPhase, p.combatPhase, p.secondMain, p.endStep)
	for i, a := range steps {
		result = append(result, fmt.Sprintf("%v. %v", i+1, a.Pretty()))
	}
	return result
}
