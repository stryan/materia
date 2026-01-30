package loader

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

type ManifestLoadStage struct {
	manager    components.ComponentReader
	overrides  []*manifests.ComponentManifest
	extensions []*manifests.ComponentManifest
}

func (s *ManifestLoadStage) Process(ctx context.Context, comp *components.Component) error {
	if comp.Version != components.DefaultComponentVersion {
		log.Debugf("skipping component manifest loading due to version mismatch: %v != %v", comp.Version, components.DefaultComponentVersion)
		// trying to load a manifest with a different version that materia knows, return empty
		return nil
	}

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
			manifest, err = manifests.MergeComponentManifests(manifest, extension)
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
