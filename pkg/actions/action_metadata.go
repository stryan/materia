package actions

import "io/fs"

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
