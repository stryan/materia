package materia

import (
	"errors"
	"fmt"
)

type ActionType int

const (
	ActionUnknown ActionType = iota
	ActionInstallComponent
	ActionRemoveComponent
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
