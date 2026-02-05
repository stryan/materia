package source

import (
	"context"
	"errors"
)

type Source interface {
	Sync(context.Context) error
	Close(context.Context) error
	Clean() error
}

type SourceConfig struct {
	URL  string `toml:"url" json:"url" yaml:"url"`
	Kind string `toml:"kind" json:"kind" yaml:"kind"`
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
