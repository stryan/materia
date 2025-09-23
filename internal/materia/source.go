package materia

import (
	"context"
	"errors"

	"github.com/knadh/koanf/v2"
)

type Source interface {
	Sync(context.Context) error
	Close(context.Context) error
	Clean() error
}

type SourceConfig struct {
	URL    string `toml:"URL" json:"URL" yaml:"URL"`
	NoSync bool   `toml:"no_sync" json:"no_sync" yaml:"no_sync"`
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

func NewSourceConfig(k *koanf.Koanf) (*SourceConfig, error) {
	var c SourceConfig
	c.URL = k.String("URL")
	c.NoSync = k.Bool("no_sync")
	return &c, nil
}
