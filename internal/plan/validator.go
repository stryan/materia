package plan

import (
	"fmt"
	"slices"

	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/pkg/components"
)

type ValidationStep interface {
	Validate(plan []actions.Action) error
}

type PlanValidatorPipeline struct {
	stages []ValidationStep
}

func (p *PlanValidatorPipeline) Validate(plan *Plan) error {
	steps := plan.Steps()
	for _, v := range p.stages {
		if err := v.Validate(steps); err != nil {
			return err
		}
	}
	return nil
}

func NewDefaultValidationPipeline(installedComponents []string) *PlanValidatorPipeline {
	return &PlanValidatorPipeline{
		stages: []ValidationStep{
			&ReloadValidator{},
			&VolumeDumpValidator{},
			&ComponentInstallValidator{installedComponents},
			&QuadletValidator{},
			&CombinedResourceValidator{},
		},
	}
}

type ReloadValidator struct{}

func (s *ReloadValidator) Validate(steps []actions.Action) error {
	needReload := false
	reload := false
	currentStep := 1
	maxSteps := len(steps)

	for _, a := range steps {
		if (a.Target.Kind == components.ResourceTypeService || a.Target.IsQuadlet()) && a.Todo == actions.ActionInstall {
			needReload = true
		}

		if a.Todo == actions.ActionReload && a.Target.Path == "" {
			reload = true
		}

		currentStep++
	}
	if needReload && !reload {
		return fmt.Errorf("invalid plan: %v/%v: systemd units added without a daemon-reload", currentStep, maxSteps) // yeah yeah this is always at the end
	}
	return nil
}

type CombinedResourceValidator struct{}

func (s *CombinedResourceValidator) Validate(steps []actions.Action) error {
	currentStep := 1
	maxSteps := len(steps)
	for _, a := range steps {
		if a.Target.Kind == components.ResourceTypeCombined {
			return fmt.Errorf("%v/%v invalid plan: tried to act on a combined resource: %v", currentStep, maxSteps, a.Target.Path)
		}
		currentStep++
	}
	return nil
}

type QuadletValidator struct{}

func (s *QuadletValidator) Validate(steps []actions.Action) error {
	currentStep := 1
	maxSteps := len(steps)
	for _, a := range steps {
		if a.Target.IsQuadlet() && a.Target.HostObject == "" {
			return fmt.Errorf("%v/%v: tried to operate on a quadlet without a backing podman object: %v", currentStep, maxSteps, a.Target)
		}
		currentStep++
	}
	return nil
}

type VolumeDumpValidator struct{}

func (v *VolumeDumpValidator) Validate(steps []actions.Action) error {
	deletedVoles := []string{}
	currentStep := 1
	maxSteps := len(steps)
	for _, a := range steps {
		if a.Todo == actions.ActionRemove && a.Target.Kind == components.ResourceTypeVolume {
			deletedVoles = append(deletedVoles, a.Target.Path)
		}
		if a.Todo == actions.ActionDump && a.Target.Kind == components.ResourceTypeVolume {
			if slices.Contains(deletedVoles, a.Target.Path) {
				return fmt.Errorf("%v/%v: invalid plan: deleted volume %v before dumping", currentStep, maxSteps, a.Target.Path)
			}
		}
		currentStep++
	}
	return nil
}

type ComponentInstallValidator struct {
	existingComponents []string
}

func (s *ComponentInstallValidator) Validate(steps []actions.Action) error {
	componentList := append([]string{}, s.existingComponents...)
	currentStep := 1
	maxSteps := len(steps)
	for _, a := range steps {

		if a.Target.Kind == components.ResourceTypeCombined {
			return fmt.Errorf("%v/%v invalid plan: tried to act on a combined resource: %v", currentStep, maxSteps, a.Target.Path)
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
	return nil
}
