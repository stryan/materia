package materia

import (
	"errors"
	"fmt"
	"strings"
)

//go:generate stringer -type ActionType -trimprefix Action
type ActionType int

//go:generate stringer -type ActionCategory -trimprefix Action
type ActionCategory int

const (
	ActionCategoryInstall ActionCategory = iota
	ActionCategoryUpdate
	ActionCategoryRemove
	ActionCategoryOther
)

const (
	ActionUnknown ActionType = iota
	ActionInstallComponent
	ActionSetupComponent
	ActionRemoveComponent
	ActionCleanupComponent
	ActionReloadUnits
	ActionEnableService
	ActionStartService
	ActionDisableService
	ActionStopService
	ActionRestartService
	ActionEnsureVolume
	ActionInstallVolumeFile
	ActionUpdateVolumeFile
	ActionRemoveVolumeFile

	ActionInstallFile
	ActionInstallQuadlet
	ActionInstallScript
	ActionInstallService
	ActionInstallComponentScript

	ActionUpdateFile
	ActionUpdateQuadlet
	ActionUpdateScript
	ActionUpdateService
	ActionUpdateComponentScript

	ActionRemoveFile
	ActionRemoveQuadlet
	ActionRemoveScript
	ActionRemoveService
	ActionRemoveComponentScript
)

type Action struct {
	Todo    ActionType
	Parent  *Component
	Payload Resource
	Content any
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
	case ActionInstallFile, ActionInstallQuadlet, ActionInstallScript, ActionInstallService, ActionInstallComponentScript:
		act := "Installing"
		if a.Payload.Template {
			act = "Templating"
		}
		return fmt.Sprintf("%v %v resource %v/%v", act, strings.ToLower(a.Payload.Kind.String()), a.Parent.Name, a.Payload.Name)
	case ActionInstallVolumeFile:
		return fmt.Sprintf("Installing volume file %v", a.Payload.Name)
	case ActionRemoveVolumeFile:
		return fmt.Sprintf("Removing volume file %v", a.Payload.Name)
	case ActionUpdateVolumeFile:
		return fmt.Sprintf("Updating volume file %v", a.Payload.Name)
	case ActionReloadUnits:
		return "Reloading systemd units"
	case ActionRemoveComponent:
		return fmt.Sprintf("Removing component %v", a.Parent.Name)
	case ActionRemoveFile, ActionRemoveQuadlet, ActionRemoveScript, ActionRemoveService, ActionRemoveComponentScript:
		return fmt.Sprintf("Removing resource %v/%v", a.Parent.Name, a.Payload.Name)
	case ActionRestartService:
		return fmt.Sprintf("Restarting service %v", a.Payload.Name)
	case ActionStartService:
		return fmt.Sprintf("Starting service %v", a.Payload.Name)
	case ActionStopService:
		return fmt.Sprintf("Stopping service %v", a.Payload.Name)
	case ActionUnknown:
		return "Unknown action"
	case ActionUpdateFile, ActionUpdateQuadlet, ActionUpdateScript, ActionUpdateService, ActionUpdateComponentScript:
		return fmt.Sprintf("Updating resource %v/%v", a.Parent.Name, a.Payload.Name)
	case ActionSetupComponent:
		return fmt.Sprintf("Setting up component %v", a.Parent.Name)
	case ActionCleanupComponent:
		return fmt.Sprintf("Cleaning up component %v", a.Parent.Name)
	default:
		panic(fmt.Sprintf("unexpected materia.ActionType: %#v", a.Todo))
	}
}

func (a *Action) Category() ActionCategory {
	switch a.Todo {
	case ActionInstallComponent, ActionInstallFile, ActionInstallQuadlet, ActionInstallScript, ActionInstallService, ActionInstallComponentScript, ActionInstallVolumeFile:
		return ActionCategoryInstall
	case ActionRemoveFile, ActionRemoveQuadlet, ActionRemoveScript, ActionRemoveService, ActionRemoveComponentScript, ActionRemoveVolumeFile, ActionRemoveComponent:
		return ActionCategoryRemove
	case ActionUpdateFile, ActionUpdateQuadlet, ActionUpdateScript, ActionUpdateService, ActionUpdateComponentScript, ActionUpdateVolumeFile:
		return ActionCategoryUpdate
	default:
		return ActionCategoryOther
	}
}
