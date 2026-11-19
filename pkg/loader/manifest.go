package loader

import (
	"context"
	"fmt"

	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

type ManifestLoadStage struct {
	manager    components.ComponentReader
	overrides  []*manifests.ComponentManifest
	extensions []*manifests.ComponentManifest
}

func (s *ManifestLoadStage) Process(ctx context.Context, comp *components.Component) error {
	manifest, err := s.manager.GetManifest(comp)
	if err != nil {
		return fmt.Errorf("can't load source component %v manifest: %w", comp.Name, err)
	}
	if len(s.overrides) > 0 {
		for _, override := range s.overrides {
			manifest, err = manifests.MergeComponentManifests(manifest, override)
			if err != nil {
				return fmt.Errorf("can't load source component %v's overrides: %w", comp.Name, err)
			}
		}
	}
	if len(s.extensions) > 0 {
		for _, extension := range s.extensions {
			manifest, err = manifests.ExtendComponentManifests(manifest, extension)
			if err != nil {
				return fmt.Errorf("can't load source component %v's overrides: %w", comp.Name, err)
			}
		}
	}
	if err := comp.ApplyManifest(manifest); err != nil {
		return fmt.Errorf("can't apply source component %v manifest: %w", comp.Name, err)
	}

	return nil
}

type StateInjectorStage struct {
	state components.ComponentLifecycle
}

func (s *StateInjectorStage) Process(ctx context.Context, comp *components.Component) error {
	comp.State = s.state
	return nil
}
