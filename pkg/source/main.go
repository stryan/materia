package source

import (
	"context"
	"errors"
)

type Source interface {
	Sync(context.Context, SyncOpts) (*SyncReport, error)
	Inspect() SyncInspectReport
	Close(context.Context) error
	String() string
	Clean() error
}

type SourceConfig struct {
	URL  string `toml:"url" json:"url" yaml:"url"`
	Kind string `toml:"kind" json:"kind" yaml:"kind"`
}
type SyncOpts struct {
	Revision string
	Subpath  string
}

type SyncReport struct {
	OldRevision, NewRevision string
}

func (r SyncReport) CanRollback() bool {
	return r.OldRevision != ""
}

type SyncInspectReport struct {
	SupportsRollback bool
}

func (c SourceConfig) String() string {
	return ""
}

func (c SourceConfig) Validate() error {
	if c.URL == "" {
		return errors.New("need source URL")
	}
	return nil
}
