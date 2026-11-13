package commands

import (
	"fmt"

	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/materia"
	"primamateria.systems/materia/pkg/source"
	"primamateria.systems/materia/pkg/source/git"
	"primamateria.systems/materia/pkg/source/local"
	"primamateria.systems/materia/pkg/source/oci"
)

func BuildSource(k *koanf.Koanf) (source.Source, error) {
	c, err := materia.NewConfig(k)
	if err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}
	sourceDir := c.SourceDir
	rawSourceConfig := k.Cut("source")
	var sourceConfig source.SourceConfig
	sourceConfig.URL = rawSourceConfig.String("url")
	sourceConfig.Kind = rawSourceConfig.String("kind")

	err = sourceConfig.Validate()
	if err != nil {
		return nil, err
	}
	var source source.Source
	switch sourceConfig.Kind {
	case "git":
		config, err := git.NewConfig(k, sourceDir, sourceConfig.URL)
		if err != nil {
			return nil, fmt.Errorf("error creating git config: %w", err)
		}
		source, err = git.NewGitSource(config)
		if err != nil {
			return nil, fmt.Errorf("invalid git source: %w", err)
		}
	case "local":
		config, err := local.NewConfig(k, sourceDir, sourceConfig.URL)
		if err != nil {
			return nil, fmt.Errorf("error creating local file config: %w", err)
		}
		source, err = local.NewLocalFileSource(config)
		if err != nil {
			return nil, fmt.Errorf("invalid local file source: %w", err)
		}
	case "oci":
		config, err := oci.NewConfig(k, sourceDir, sourceConfig.URL)
		if err != nil {
			return nil, fmt.Errorf("error creating OCI config: %w", err)
		}
		source, err = oci.NewOCISource(config)
		if err != nil {
			return nil, fmt.Errorf("invalid OCI source: %w", err)
		}
	default:
		return nil, fmt.Errorf("invalid source kind %v for %v", sourceConfig.Kind, sourceConfig.URL)
	}
	return source, nil
}
