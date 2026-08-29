package local

import (
	"errors"

	"github.com/knadh/koanf/v2"
)

type Config struct {
	SourcePath  string `toml:"source_path" json:"source_path" yaml:"source_path"`
	Destination string `toml:"destination" json:"destination" yaml:"destination"`
}

func NewConfig(_ *koanf.Koanf, destination, path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("need local file source path")
	}
	return &Config{
		SourcePath:  path,
		Destination: destination,
	}, nil
}
