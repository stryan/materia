package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

func LoadConfigs(_ context.Context, configFile string, cliflags map[string]any) (*koanf.Koanf, error) {
	k := koanf.New(".")
	fileConf := koanf.New(".")
	envConf := koanf.New(".")
	cliConf := koanf.New(".")
	if configFile != "" {
		err := fileConf.Load(file.Provider(configFile), toml.Parser())
		if err != nil {
			return nil, fmt.Errorf("error loading config file: %w", err)
		}
	}
	err := envConf.Load(env.Provider("MATERIA", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(
			strings.TrimPrefix(s, "MATERIA_")), "__", ".")
	}), nil)
	if err != nil {
		return nil, fmt.Errorf("error loading config from env: %w", err)
	}
	err = cliConf.Load(confmap.Provider(cliflags, "."), nil)
	if err != nil {
		return nil, err
	}
	err = k.Merge(fileConf)
	if err != nil {
		return nil, fmt.Errorf("error building config: %w", err)
	}
	err = k.Merge(envConf)
	if err != nil {
		return nil, fmt.Errorf("error building config: %w", err)
	}
	err = k.Merge(cliConf)
	if err != nil {
		return nil, fmt.Errorf("error building config: %w", err)
	}

	return k, err
}
