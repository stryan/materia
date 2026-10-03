package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

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

	ActionExecute
)

func (t ActionType) IsServiceAction() bool {
	return t == ActionStart || t == ActionRestart || t == ActionStop || t == ActionReload || t == ActionEnable || t == ActionDisable
}

func (t ActionType) IsResourceAction() bool {
	return t == ActionInstall || t == ActionRemove || t == ActionUpdate
}

func (t ActionType) IsHostAction() bool {
	return t == ActionSetup || t == ActionCleanup || t == ActionMount || t == ActionImport || t == ActionDump
}

type Action struct {
	Todo        ActionType            `json:"todo" toml:"todo"`
	Parent      *components.Component `json:"parent" toml:"parent"`
	Target      components.Resource   `json:"target" toml:"target"`
	DiffContent []diffmatchpatch.Diff `json:"content" toml:"content"`
	Priority    int                   `json:"priority" toml:"priority"`
	Metadata    *ActionMetadata       `json:"metadata,omitempty" toml:"metadata,omitempty"`
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
		name = a.Parent.InstanceName()
	}
	return fmt.Sprintf("{a %v %v %v }", a.Todo, name, a.Target.Path)
}

func (a *Action) Pretty() string {
	name := "<parent>"
	if a.Parent != nil {
		name = a.Parent.InstanceName()
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

type ActionMetadata struct {
	ServiceTimeout    *int         `json:"service_timeout,omitempty" toml:"service_timeout,omitempty"`
	ServiceUntilState *string      `json:"service_until_state,omitempty" toml:"service_until_state,omitempty"`
	Command           *string      `json:"command,omitempty" toml:"command,omitempty"`
	VolumeName        *string      `json:"volume_name,omitempty" toml:"volume_name,omitempty"`
	OneshotName       *string      `json:"oneshot_name,omitempty" toml:"oneshot_name,omitempty"`
	PrevMode          *fs.FileMode `toml:"prev_mode,omitempty" json:"prev_mode,omitempty"`
}

type MetadataOption func(*ActionMetadata)

func WithPreviousMode(m fs.FileMode) MetadataOption {
	return func(md *ActionMetadata) { md.PrevMode = new(m) }
}

func WithServiceTimeout(seconds int) MetadataOption {
	return func(md *ActionMetadata) { md.ServiceTimeout = new(seconds) }
}

func WithOneshotName(name string) MetadataOption {
	return func(md *ActionMetadata) { md.OneshotName = new(name) }
}

func WithServiceUntilState(state string) MetadataOption {
	return func(md *ActionMetadata) { md.ServiceUntilState = new(state) }
}

func WithCommand(cmd string) MetadataOption {
	return func(md *ActionMetadata) { md.Command = new(cmd) }
}

func WithVolumeName(name string) MetadataOption {
	return func(md *ActionMetadata) { md.VolumeName = new(name) }
}

func (a Action) With(opts ...MetadataOption) Action {
	var md ActionMetadata
	if a.Metadata != nil {
		md = *a.Metadata
	}
	for _, opt := range opts {
		opt(&md)
	}
	a.Metadata = &md
	return a
}

func (a Action) GetPrevMode() (fs.FileMode, bool) {
	if a.Metadata == nil || a.Metadata.PrevMode == nil {
		return 0, false
	}
	return *a.Metadata.PrevMode, true
}

func (a Action) GetServiceTimeout() (int, bool) {
	if a.Metadata == nil || a.Metadata.ServiceTimeout == nil {
		return 0, false
	}
	return *a.Metadata.ServiceTimeout, true
}

func (a Action) GetOneshotName() (string, bool) {
	if a.Metadata == nil || a.Metadata.OneshotName == nil {
		return "", false
	}
	return *a.Metadata.OneshotName, true
}

func (a Action) GetServiceUntilState() (string, bool) {
	if a.Metadata == nil || a.Metadata.ServiceUntilState == nil {
		return "", false
	}
	return *a.Metadata.ServiceUntilState, true
}

func (a Action) GetCommand() (string, bool) {
	if a.Metadata == nil || a.Metadata.Command == nil {
		return "", false
	}
	return *a.Metadata.Command, true
}

func (a Action) GetVolumeName() (string, bool) {
	if a.Metadata == nil || a.Metadata.VolumeName == nil {
		return "", false
	}
	return *a.Metadata.VolumeName, true
}
