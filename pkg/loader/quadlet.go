package loader

import (
	"context"
	"fmt"

	"primamateria.systems/materia/pkg/components"
)

type QuadletObjectExtractorStage struct{}

func (s *QuadletObjectExtractorStage) Process(ctx context.Context, comp *components.Component) error {
	for _, r := range comp.Resources.List() {
		if r.IsQuadlet() {
			hostObject, err := r.GetHostObject(r.Content)
			if err != nil {
				return err
			}
			r.HostObject = hostObject
			comp.Resources.Set(r)
		}
	}
	return nil
}

type QuadletExpanderStage struct{}

func (s *QuadletExpanderStage) Process(ctx context.Context, comp *components.Component) error {
	for _, r := range comp.Resources.List() {
		if r.Kind == components.ResourceTypeCombined {
			expandedResources, err := components.GetResourcesFromQuadletsFile(r.Parent, r.Content)
			if err != nil {
				return fmt.Errorf("can't expand combined resource %v: %w", r.Path, err)
			}
			for _, er := range expandedResources {
				// Since range copies the value these won't get processed in this loop
				// Combined quadlets can only have quadlets and data files so there's no other processing to be done
				comp.Resources.Set(er)
			}
			// Remove the combined resource from the set so we don't accidentally install it
			comp.Resources.Delete(r.Path)
		}
	}
	return nil
}
