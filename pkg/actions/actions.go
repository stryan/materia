package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/pkg/components"
)

type Action struct {
	Todo        ActionType            `json:"todo" toml:"todo"`
	Parent      string                `json:"parent" toml:"parent"`
	Target      components.Resource   `json:"target" toml:"target"`
	DiffContent []diffmatchpatch.Diff `json:"content" toml:"content"`
	Priority    int                   `json:"priority" toml:"priority"`
	Metadata    *ActionMetadata       `json:"metadata,omitempty" toml:"metadata,omitempty"`
}

func (a Action) Validate() error {
	if a.Todo == ActionUnknown {
		return errors.New("unknown action")
	}
	if a.Parent == "" {
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

func (a Action) String() string {
	name := "<parent>"
	if a.Parent != "" {
		name = a.Parent
	}
	return fmt.Sprintf("{a %v %v %v }", a.Todo, name, a.Target.Path)
}

func (a *Action) Pretty() string {
	name := "<parent>"
	if a.Parent != "" {
		name = a.Parent
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
		Parent:   a.Parent,
		Target:   a.Target,
		Priority: a.Priority,
	})
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
