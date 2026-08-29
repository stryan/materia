package manifests

import (
	"errors"
	"maps"
	"slices"

	"github.com/BurntSushi/toml"
)

var ComponentManifestFile = "MANIFEST.toml"

type ServiceResourceConfig struct {
	Service     string   `toml:"Service"`
	RestartedBy []string `toml:"RestartedBy"`
	ReloadedBy  []string `toml:"ReloadedBy"`
	Disabled    bool     `toml:"Disabled"`
	Static      bool     `toml:"Static"`
	Stopped     bool     `toml:"Stopped"`
	Oneshot     bool     `toml:"Oneshot"`
	Timeout     int      `toml:"Timeout"`
}

type Settings struct {
	NoRestart     *bool   `toml:"NoRestart"`
	NoExpansion   *bool   `toml:"NoExpansion"`
	SetupScript   *string `toml:"SetupScript"`
	CleanupScript *string `toml:"CleanupScript"`
	PreScript     *string `toml:"PreScript"`
	PostScript    *string `toml:"PostScript"`
}

func (s *Settings) Merge(o Settings) {
	if o.NoRestart != nil {
		s.NoRestart = o.NoRestart
	}
	if o.NoExpansion != nil {
		s.NoExpansion = o.NoExpansion
	}
	if o.SetupScript != nil {
		s.SetupScript = o.SetupScript
	}
	if o.CleanupScript != nil {
		s.CleanupScript = o.CleanupScript
	}
	if o.PreScript != nil {
		s.PreScript = o.PreScript
	}
	if o.PostScript != nil {
		s.PostScript = o.PostScript
	}
}

func (s Settings) GetNoRestart() bool {
	if s.NoRestart == nil {
		return false
	}
	return *s.NoRestart
}

func (s Settings) GetNoExpansion() bool {
	if s.NoExpansion == nil {
		return false
	}
	return *s.NoExpansion
}

func (s Settings) GetSetupScript() string {
	if s.SetupScript == nil {
		return ""
	}
	return *s.SetupScript
}

func (s Settings) GetCleanupScript() string {
	if s.CleanupScript == nil {
		return ""
	}
	return *s.CleanupScript
}

func (s Settings) GetPreScript() string {
	if s.PreScript == nil {
		return ""
	}
	return *s.PreScript
}

func (s Settings) GetPostScript() string {
	if s.PostScript == nil {
		return ""
	}
	return *s.PostScript
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
	var c ComponentManifest
	_, err := toml.Decode(string(buffer), &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func LoadComponentManifestFromFile(path string) (*ComponentManifest, error) {
	var c ComponentManifest
	_, err := toml.DecodeFile(path, &c)
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
	result.Snippets = original.Snippets
	result.Scripts = original.Scripts
	result.Services = original.Services
	result.Secrets = original.Secrets

	if len(override.Defaults) > 0 {
		result.Defaults = maps.Clone(override.Defaults)
	} else {
		result.Defaults = maps.Clone(original.Defaults)
	}
	result.Settings.Merge(override.Settings)
	if len(override.Snippets) > 0 {
		result.Snippets = slices.Clone(override.Snippets)
	}
	if len(override.Services) > 0 {
		result.Services = slices.Clone(override.Services)
	}
	if len(override.Scripts) > 0 {
		result.Scripts = slices.Clone(override.Scripts)
	}
	if len(override.Secrets) > 0 {
		result.Secrets = slices.Clone(override.Secrets)
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
