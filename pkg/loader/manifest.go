package loader

import (
	"context"
	"fmt"

	"charm.land/log/v2"
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

	manifest, err := mergedManifest(s.manager, comp, s.overrides, s.extensions)
	if err != nil {
		return err
	}
	if err := comp.ApplyManifest(manifest); err != nil {
		return fmt.Errorf("can't apply source component %v manifest: %w", comp.Name, err)
	}

	return nil
}

// mergedManifest loads a component's manifest and applies overrides, then extensions.
func mergedManifest(manager components.ComponentReader, comp *components.Component, overrides, extensions []*manifests.ComponentManifest) (*manifests.ComponentManifest, error) {
	manifest, err := manager.GetManifest(comp)
	if err != nil {
		return nil, fmt.Errorf("can't load source component %v manifest: %w", comp.Name, err)
	}
	for _, override := range overrides {
		manifest, err = manifests.MergeComponentManifests(manifest, override)
		if err != nil {
			return nil, fmt.Errorf("can't load source component %v's overrides: %w", comp.Name, err)
		}
	}
	for _, extension := range extensions {
		manifest, err = manifests.ExtendComponentManifests(manifest, extension)
		if err != nil {
			return nil, fmt.Errorf("can't load source component %v's extensions: %w", comp.Name, err)
		}
	}
	return manifest, nil
}

type StateInjectorStage struct {
	state components.ComponentLifecycle
}

func (s *StateInjectorStage) Process(ctx context.Context, comp *components.Component) error {
	comp.State = s.state
	return nil
}
