package file

import (
	"errors"

	"github.com/knadh/koanf/v2"
)

type Config struct {
	SourcePath  string
	Destination string
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
