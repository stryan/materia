package manifests

import (
	"errors"
	"maps"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var ComponentManifestFile = "MANIFEST.toml"

type ServiceResourceConfig struct {
	Service     string   `toml:"Service"`
	RestartedBy []string `toml:"RestartedBy"`
	ReloadedBy  []string `toml:"ReloadedBy"`
	Disabled    bool     `toml:"Disabled"`
	Static      bool     `toml:"Static"`
}

type BackupsConfig struct {
	Online     bool     `toml:"Online"`
	Pause      bool     `toml:"Pause"`
	Skip       []string `toml:"Skip"`
	NoCompress bool     `toml:"NoCompress"`
}

type Settings struct {
	NoRestart bool
}

func (src ServiceResourceConfig) Validate() error {
	if src.Service == "" {
		return errors.New("service config without a name")
	}
	return nil
}

type ComponentManifest struct {
	Defaults map[string]any          `toml:"Defaults"`
	Settings Settings                `toml:"Settings"`
	Snippets []SnippetConfig         `toml:"Snippets"`
	Services []ServiceResourceConfig `toml:"Services"`
	Backups  *BackupsConfig          `toml:"Backups"`
	Scripts  []string                `toml:"Scripts"`
	Secrets  []string                `toml:"Secrets"`
}

func LoadComponentManifest(path string) (*ComponentManifest, error) {
	k := koanf.New(".")
	err := k.Load(file.Provider(path), toml.Parser())
	if err != nil {
		return nil, err
	}
	var c ComponentManifest
	err = k.Unmarshal("", &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func MergeComponentManifests(left, right *ComponentManifest) (*ComponentManifest, error) {
	if left == nil {
		return nil, errors.New("need non nil left manifest for merge")
	}
	if right == nil {
		return nil, errors.New("need non nil right manifest for merge")
	}
	result := ComponentManifest{}

	if len(left.Defaults) > 0 {
		result.Defaults = maps.Clone(left.Defaults)
	} else {
		result.Defaults = maps.Clone(right.Defaults)
	}
	if len(left.Snippets) > 0 {
		result.Snippets = append(result.Snippets, left.Snippets...)
	} else {
		result.Snippets = append(result.Snippets, right.Snippets...)
	}
	if len(left.Services) > 0 {
		result.Services = append(result.Services, left.Services...)
	} else {
		copy(result.Services, right.Services)
		result.Services = append(result.Services, right.Services...)
	}
	if left.Backups != nil {
		result.Backups = left.Backups
	} else {
		result.Backups = right.Backups
	}
	if len(left.Scripts) > 0 {
		result.Scripts = append(result.Scripts, left.Scripts...)
	} else {
		result.Scripts = append(result.Scripts, right.Scripts...)
	}
	if len(left.Secrets) > 0 {
		result.Secrets = append(result.Secrets, left.Secrets...)
	} else {
		result.Secrets = append(result.Secrets, right.Secrets...)
		copy(result.Secrets, right.Secrets)
	}

	return &result, nil
}
