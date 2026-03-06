package file

import (
	"errors"

	"github.com/knadh/koanf/v2"
)

type Config struct {
	SourcePath  string `toml:"source_path" koanf:"source_path"`
	Destination string `toml:"destination" koanf:"destination"`
}

func NewConfig(_ *koanf.Koanf, destination, path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("need file source path")
	}
	return &Config{
		SourcePath:  path,
		Destination: destination,
	}, nil
}
