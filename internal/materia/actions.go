package materia

import (
	"errors"
	"fmt"

	"primamateria.systems/materia/internal/components"
)

//go:generate stringer -type ActionType -trimprefix Action
type ActionType int

const (
	ActionUnknown ActionType = iota

	ActionInstall
	ActionRemove
	ActionUpdate

	ActionStart
	ActionStop
	ActionRestart
	ActionReload
	ActionEnable
	ActionDisable

	ActionEnsure
	ActionSetup
	ActionCleanup

	ActionMount
	ActionImport
	ActionDump
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
	if err := a.Payload.Validate(); err != nil {
		return fmt.Errorf("invalid payload %v for action: %w", a.Payload, err)
	}
	return nil
}

func (a *Action) String() string {
	return fmt.Sprintf("{a %v %v %v }", a.Todo, a.Parent.Name, a.Payload.Path)
}

func (a *Action) Pretty() string {
	return fmt.Sprintf("%v %v %v", a.Todo, a.Payload.Kind, a.Payload.Path)
}
