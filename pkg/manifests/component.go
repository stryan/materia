package manifests

import (
	"errors"
	"maps"
	"slices"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
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

type Settings struct {
	NoRestart     bool   `toml:"NoRestart"`
	SetupScript   string `toml:"SetupScript"`
	CleanupScript string `toml:"CleanupScript"`
}

func (s *Settings) Merge(o Settings) {
	s.NoRestart = o.NoRestart
	s.CleanupScript = o.CleanupScript
	s.SetupScript = o.SetupScript
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
	Scripts  []string                `toml:"Scripts"`
	Secrets  []string                `toml:"Secrets"`
}

func LoadComponentManifestFromContent(buffer []byte) (*ComponentManifest, error) {
	k := koanf.New(".")
	err := k.Load(rawbytes.Provider(buffer), toml.Parser())
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

func LoadComponentManifestFromFile(path string) (*ComponentManifest, error) {
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
	result.Settings.Merge(override.Settings)
	if len(override.Snippets) > 0 {
		copy(result.Snippets, override.Snippets)
	} else {
		copy(result.Snippets, original.Snippets)
	}
	if len(override.Services) > 0 {
		copy(result.Services, override.Services)
	} else {
		copy(result.Services, original.Services)
	}

	if len(override.Scripts) > 0 {
		copy(result.Scripts, override.Scripts)
	} else {
		copy(result.Scripts, original.Scripts)
	}
	if len(override.Secrets) > 0 {
		result.Secrets = append(result.Secrets, override.Secrets...)
		copy(result.Secrets, override.Secrets)
	} else {
		copy(result.Secrets, original.Secrets)
	}

	return &result, nil
}

func ExtendComponentManifests(original, extension *ComponentManifest) (*ComponentManifest, error) {
	if original == nil {
		return nil, errors.New("need non nil original manifest for merge")
	}
	if extension == nil {
		return nil, errors.New("need non nil extension manifest for merge")
	}

	result := ComponentManifest{}
	result.Defaults = maps.Clone(original.Defaults)
	if len(extension.Defaults) > 0 {
		maps.Copy(result.Defaults, extension.Defaults)
	}
	result.Settings = original.Settings
	result.Settings.Merge(extension.Settings)
	result.Services = original.Services
	for _, s := range extension.Services {
		i := slices.IndexFunc(result.Services, func(src ServiceResourceConfig) bool {
			return src.Service == s.Service
		})
		if i == -1 {
			result.Services = append(result.Services, s)
		} else {
			result.Services[i].ReloadedBy = append(result.Services[i].ReloadedBy, s.ReloadedBy...)
			result.Services[i].RestartedBy = append(result.Services[i].RestartedBy, s.RestartedBy...)
		}
	}
	result.Snippets = append(original.Snippets, extension.Snippets...)
	result.Scripts = append(original.Scripts, extension.Scripts...)
	result.Secrets = append(original.Secrets, extension.Secrets...)

	return &result, nil
}
