package materia

import (
	"errors"
	"fmt"
	"strings"

	"primamateria.systems/materia/internal/components"
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
	ActionUpdateComponent
	ActionReloadUnits
	ActionEnableService
	ActionStartService
	ActionDisableService
	ActionStopService
	ActionRestartService
	ActionReloadService
	ActionEnsureVolume
	ActionInstallVolumeFile
	ActionUpdateVolumeFile
	ActionRemoveVolumeFile
	ActionInstallDirectory
	ActionRemoveDirectory

	ActionInstallFile
	ActionInstallQuadlet
	ActionInstallScript
	ActionInstallService
	ActionInstallComponentScript
	ActionInstallPodmanSecret

	ActionUpdateFile
	ActionUpdateQuadlet
	ActionUpdateScript
	ActionUpdateService
	ActionUpdateComponentScript
	ActionUpdatePodmanSecret

	ActionRemoveFile
	ActionRemoveQuadlet
	ActionRemoveScript
	ActionRemoveService
	ActionRemoveComponentScript
	ActionRemovePodmanSecret
)

type Action struct {
	Todo    ActionType
	Parent  *components.Component
	Payload components.Resource
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
	case ActionInstallFile, ActionInstallQuadlet, ActionInstallScript, ActionInstallService, ActionInstallComponentScript, ActionInstallDirectory:
		act := "Installing"
		if a.Payload.Template {
			act = "Templating"
		}
		return fmt.Sprintf("%v %v resource %v/%v", act, strings.ToLower(a.Payload.Kind.String()), a.Parent.Name, a.Payload.Path)
	case ActionInstallVolumeFile:
		return fmt.Sprintf("Installing volume file %v", a.Payload.Path)
	case ActionRemoveVolumeFile:
		return fmt.Sprintf("Removing volume file %v", a.Payload.Path)
	case ActionUpdateVolumeFile:
		return fmt.Sprintf("Updating volume file %v", a.Payload.Path)
	case ActionInstallPodmanSecret:
		return fmt.Sprintf("Installing podman secret %v", a.Payload.Name)
	case ActionUpdatePodmanSecret:
		return fmt.Sprintf("Updating podman secret %v", a.Payload.Name)
	case ActionRemovePodmanSecret:
		return fmt.Sprintf("Removing podman secret %v", a.Payload.Name)
	case ActionReloadUnits:
		return "Reloading systemd units"
	case ActionRemoveComponent:
		return fmt.Sprintf("Removing component %v", a.Parent.Name)
	case ActionRemoveFile, ActionRemoveQuadlet, ActionRemoveScript, ActionRemoveService, ActionRemoveComponentScript, ActionRemoveDirectory:
		return fmt.Sprintf("Removing resource %v/%v", a.Parent.Name, a.Payload.Path)
	case ActionRestartService:
		return fmt.Sprintf("Restarting service %v/%v", a.Parent.Name, a.Payload.Path)
	case ActionStartService:
		return fmt.Sprintf("Starting service %v/%v", a.Parent.Name, a.Payload.Path)
	case ActionStopService:
		return fmt.Sprintf("Stopping service %v/%v", a.Parent.Name, a.Payload.Path)
	case ActionEnableService:
		return fmt.Sprintf("Enabling service %v/%v", a.Parent.Name, a.Payload.Path)
	case ActionDisableService:
		return fmt.Sprintf("Disabling service %v/%v", a.Parent.Name, a.Payload.Path)
	case ActionUnknown:
		return "Unknown action"
	case ActionUpdateFile, ActionUpdateQuadlet, ActionUpdateScript, ActionUpdateService, ActionUpdateComponentScript:
		return fmt.Sprintf("Updating resource %v/%v", a.Parent.Name, a.Payload.Path)
	case ActionSetupComponent:
		return fmt.Sprintf("Setting up component %v", a.Parent.Name)
	case ActionCleanupComponent:
		return fmt.Sprintf("Cleaning up component %v", a.Parent.Name)
	case ActionUpdateComponent:
		return fmt.Sprintf("Updating component %v", a.Parent.Name)
	default:
		panic(fmt.Sprintf("unexpected materia.ActionType: %#v", a.Todo))
	}
}

func (a *Action) Category() ActionCategory {
	switch a.Todo {
	case ActionInstallComponent, ActionInstallFile, ActionInstallQuadlet, ActionInstallScript, ActionInstallService, ActionInstallComponentScript, ActionInstallVolumeFile, ActionInstallPodmanSecret:
		return ActionCategoryInstall
	case ActionRemoveFile, ActionRemoveQuadlet, ActionRemoveScript, ActionRemoveService, ActionRemoveComponentScript, ActionRemoveVolumeFile, ActionRemoveComponent, ActionRemovePodmanSecret:
		return ActionCategoryRemove
	case ActionUpdateFile, ActionUpdateQuadlet, ActionUpdateScript, ActionUpdateService, ActionUpdateComponentScript, ActionUpdateVolumeFile, ActionUpdatePodmanSecret:
		return ActionCategoryUpdate
	default:
		return ActionCategoryOther
	}
}
