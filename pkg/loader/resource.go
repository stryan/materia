package loader

import (
	"context"
	"fmt"

	"primamateria.systems/materia/pkg/components"
)

type ComponentInitStage struct {
	manager components.ComponentReader
}

func (s *ComponentInitStage) Process(ctx context.Context, comp *components.Component) error {
	newcomp, err := s.manager.GetComponent(comp.Name)
	if err != nil {
		return fmt.Errorf("can't load host component: %w", err)
	}
	*comp = *newcomp
	return nil
}

type ResourceDiscoveryStage struct {
	manager components.ComponentReader
}

func (s *ResourceDiscoveryStage) Process(ctx context.Context, comp *components.Component) error {
	for _, r := range comp.Resources.List() {
		if r.Kind == components.ResourceTypePodmanSecret {
			r.Content = "<UNFILLED_SECRET>"
			comp.Resources.Set(r)
			continue
		}
		bodyTemplate, err := s.manager.ReadResource(r)
		if err != nil {
			return fmt.Errorf("can't read source resource %v/%v: %w", comp.Name, r.Name(), err)
		}
		r.Content = bodyTemplate
		comp.Resources.Set(r)
	}

	return nil
}

type AppCompatibilityStage struct{}

func (s *AppCompatibilityStage) Process(ctx context.Context, comp *components.Component) error {
	appfileData := comp.ToAppfile()
	appFile := components.Resource{
		Path:    fmt.Sprintf(".%v.app", comp.Name),
		Parent:  comp.Name,
		Kind:    components.ResourceTypeAppFile,
		Content: string(appfileData),
	}
	return comp.Resources.Add(appFile)
}
