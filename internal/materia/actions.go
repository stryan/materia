package materia

import (
	"errors"
	"fmt"
)

//go:generate stringer -type ActionType -trimprefix Action
type ActionType int

const (
	ActionUnknown ActionType = iota
	ActionInstallComponent
	ActionSetupComponent
	ActionRemoveComponent
	ActionCleanupComponent
	ActionReloadUnits
	ActionStartService
	ActionStopService
	ActionRestartService
	ActionInstallResource
	ActionInstallVolumeResource
	ActionUpdateResource
	ActionRemoveResource
)

type Action struct {
	Todo    ActionType
	Parent  *Component
	Payload Resource
}

func (a Action) Validate() error {
	if a.Todo == ActionUnknown {
		return errors.New("unknown action")
	}
	if a.Parent == nil {
		return errors.New("action without parent")
	}
	return nil
}

func (a *Action) String() string {
	return fmt.Sprintf("{a %v %v %v }", a.Todo, a.Parent.Name, a.Payload.Name)
}

func (a *Action) Pretty() string {
	switch a.Todo {
	case ActionInstallComponent:
		return fmt.Sprintf("Installing component %v", a.Parent.Name)
	case ActionInstallResource:
		return fmt.Sprintf("Installing resource %v/%v", a.Parent.Name, a.Payload.Name)
	case ActionInstallVolumeResource:
		return fmt.Sprintf("Installing volume resource %v", a.Payload.Name)
	case ActionReloadUnits:
		return "Reloading systemd units"
	case ActionRemoveComponent:
		return fmt.Sprintf("Removing component %v", a.Parent.Name)
	case ActionRemoveResource:
		return fmt.Sprintf("Removing resource %v/%v", a.Parent.Name, a.Payload.Name)
	case ActionRestartService:
		return fmt.Sprintf("Restarting service %v", a.Payload.Name)
	case ActionStartService:
		return fmt.Sprintf("Starting service %v", a.Payload.Name)
	case ActionStopService:
		return fmt.Sprintf("Stopping service %v", a.Payload.Name)
	case ActionUnknown:
		return "Unknown action"
	case ActionUpdateResource:
		return fmt.Sprintf("Updating resource %v/%v", a.Parent.Name, a.Payload.Name)
	default:
		panic(fmt.Sprintf("unexpected materia.ActionType: %#v", a.Todo))
	}
}
