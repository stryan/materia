package manifests

import (
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var ComponentManifestFile = "MANIFEST.toml"

type ServiceResourceConfig struct {
	Service     string
	RestartedBy []string
	ReloadedBy  []string
	Disabled    bool
	Static      bool
}

type BackupsConfig struct {
	Online     bool
	Pause      bool
	Skip       []string
	NoCompress bool
}

func (src ServiceResourceConfig) Validate() error {
	return nil
}

type ComponentManifest struct {
	Defaults map[string]any
	Snippets []SnippetConfig
	Services []ServiceResourceConfig `toml:"services"`
	Backups  *BackupsConfig          `toml:"backups"`
	Scripts  []string
	Secrets  []string
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
