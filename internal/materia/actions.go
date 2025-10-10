package materia

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sergi/go-diff/diffmatchpatch"
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
	Todo     ActionType            `json:"todo" toml:"todo"`
	Parent   *components.Component `json:"parent" toml:"parent"`
	Target   components.Resource   `json:"target" toml:"target"`
	Content  any                   `json:"content" toml:"content"`
	Priority int                   `json:"priority" toml:"priority"`
}

func (a Action) Validate() error {
	if a.Todo == ActionUnknown {
		return errors.New("unknown action")
	}
	if a.Parent == nil {
		return errors.New("action without parent")
	}
	if err := a.Target.Validate(); err != nil {
		return fmt.Errorf("invalid payload %v for action: %w", a.Target, err)
	}
	if a.Todo == ActionInstall || a.Todo == ActionRemove || a.Todo == ActionUpdate {
		if a.Target.IsFile() {
			if a.Content == nil {
				return errors.New("file related action has no content")
			}
		}
	}
	return nil
}

func (a *Action) String() string {
	return fmt.Sprintf("{a %v %v %v }", a.Todo, a.Parent.Name, a.Target.Path)
}

func (a *Action) Pretty() string {
	return fmt.Sprintf("(%v) %v %v %v", a.Parent.Name, a.Todo, a.Target.Kind, a.Target.Path)
}

func (a *Action) GetContentAsDiffs() ([]diffmatchpatch.Diff, error) {
	var diffs []diffmatchpatch.Diff
	if a.Todo != ActionInstall && a.Todo != ActionRemove && a.Todo != ActionUpdate {
		return diffs, errors.New("action does not have diffs")
	}
	diffs, ok := a.Content.([]diffmatchpatch.Diff)
	if !ok {
		return diffs, errors.New("should have diffs but don't")
	}
	return diffs, nil
}

func (a *Action) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Todo     ActionType
		Parent   string
		Target   components.Resource
		Priority int
	}{
		Todo:     a.Todo,
		Parent:   a.Parent.Name,
		Target:   a.Target,
		Priority: a.Priority,
	})
}
