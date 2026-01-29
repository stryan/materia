package actions

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/pkg/components"
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

func (t ActionType) IsServiceAction() bool {
	return t == ActionStart || t == ActionRestart || t == ActionStop || t == ActionReload || t == ActionEnable || t == ActionDisable
}

func (t ActionType) IsResourceAction() bool {
	return t == ActionInstall || t == ActionRemove || t == ActionUpdate
}

func (t ActionType) IsHostAction() bool {
	return t == ActionSetup || t == ActionCleanup || t == ActionMount || t == ActionImport || t == ActionDump || t == ActionEnable
}

type Action struct {
	Todo        ActionType            `json:"todo" toml:"todo"`
	Parent      *components.Component `json:"parent" toml:"parent"`
	Target      components.Resource   `json:"target" toml:"target"`
	DiffContent []diffmatchpatch.Diff `json:"content" toml:"content"`
	Priority    int                   `json:"priority" toml:"priority"`
	Metadata    *ActionMetadata       `json:"metadata,omitempty" toml:"metadata,omitempty"`
}

type ActionMetadata struct {
	ServiceTimeout *int `json:"service_timeout,omitempty" toml:"service_timeout,omitempty"`
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
	if a.Todo == ActionUpdate {
		if a.Target.IsFile() {
			if a.DiffContent == nil {
				return fmt.Errorf("file related action has no diff: %v", a)
			}
		}
	}
	return nil
}

func (a *Action) String() string {
	name := "<parent>"
	if a.Parent != nil {
		name = a.Parent.Name
	}
	return fmt.Sprintf("{a %v %v %v }", a.Todo, name, a.Target.Path)
}

func (a *Action) Pretty() string {
	name := "<parent>"
	if a.Parent != nil {
		name = a.Parent.Name
	}
	return fmt.Sprintf("(%v) %v %v %v", name, a.Todo, a.Target.Kind, a.Target.Path)
}

func (a *Action) GetContentAsDiffs() ([]diffmatchpatch.Diff, error) {
	var diffs []diffmatchpatch.Diff
	if a.Todo != ActionInstall && a.Todo != ActionRemove && a.Todo != ActionUpdate {
		return diffs, errors.New("action does not have diffs")
	}
	return a.DiffContent, nil
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
