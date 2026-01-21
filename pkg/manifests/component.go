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
	Stopped     bool     `toml:"Stopped"`
	Timeout     int      `toml:"Timeout"`
}

type BackupsConfig struct {
	Online     bool     `toml:"Online"`
	Pause      bool     `toml:"Pause"`
	Skip       []string `toml:"Skip"`
	NoCompress bool     `toml:"NoCompress"`
}

type Settings struct {
	NoRestart     bool   `toml:"NoRestart"`
	SetupScript   string `toml:"SetupScript"`
	CleanupScript string `toml:"CleanupScript"`
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

func MergeComponentManifests(original, override *ComponentManifest) (*ComponentManifest, error) {
	if original == nil {
		return nil, errors.New("need non nil original manifest for merge")
	}
	if override == nil {
		return nil, errors.New("need non nil override manifest for merge")
	}

	result := ComponentManifest{}

	if len(override.Defaults) > 0 {
		result.Defaults = maps.Clone(override.Defaults)
	} else {
		result.Defaults = maps.Clone(original.Defaults)
	}
	if len(override.Snippets) > 0 {
		result.Snippets = append(result.Snippets, override.Snippets...)
	} else {
		result.Snippets = append(result.Snippets, original.Snippets...)
	}
	if len(override.Services) > 0 {
		result.Services = append(result.Services, override.Services...)
	} else {
		copy(result.Services, original.Services)
		result.Services = append(result.Services, original.Services...)
	}
	if override.Backups != nil {
		result.Backups = override.Backups
	} else {
		result.Backups = original.Backups
	}
	if len(override.Scripts) > 0 {
		result.Scripts = append(result.Scripts, override.Scripts...)
	} else {
		result.Scripts = append(result.Scripts, original.Scripts...)
	}
	if len(override.Secrets) > 0 {
		result.Secrets = append(result.Secrets, override.Secrets...)
	} else {
		result.Secrets = append(result.Secrets, original.Secrets...)
		copy(result.Secrets, original.Secrets)
	}

	return &result, nil
}
