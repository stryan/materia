package manifests

import (
	"errors"

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

func (src ServiceResourceConfig) Validate() error {
	if src.Service == "" {
		return errors.New("service config without a name")
	}
	return nil
}

type ComponentManifest struct {
	Defaults map[string]any          `toml:"Defaults"`
	Snippets []SnippetConfig         `toml:"Snippets"`
	Services []ServiceResourceConfig `toml:"services"`
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
