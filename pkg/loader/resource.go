package loader

import (
	"context"
	"fmt"
	"strings"

	"primamateria.systems/materia/pkg/components"
)

type ComponentInitStage struct {
	manager components.ComponentReader
}

func (s *ComponentInitStage) Process(ctx context.Context, comp *components.Component) error {
	newcomp, err := s.manager.GetComponent(comp.InstanceName())
	if err != nil {
		return fmt.Errorf("can't load component: %w", err)
	}
	if strings.Contains(comp.Name, "@") {
		split := strings.Split(comp.Name, "@")
		comp.Name = split[0]
		comp.Instance = split[1]
	}
	comp.Resources = newcomp.Resources
	comp.Version = newcomp.Version
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
			return fmt.Errorf("can't read resource %v/%v: %w", comp.Name, r.Name(), err)
		}
		r.Content = bodyTemplate
		comp.Resources.Set(r)
	}

	return nil
}

type AppCompatibilityStage struct{}

func (s *AppCompatibilityStage) Process(ctx context.Context, comp *components.Component) error {
	appfileData := comp.ToAppfile()
	name := comp.Name
	if comp.IsInstanced() {
		name = fmt.Sprintf("%v_%v", comp.Name, comp.Instance)
	}
	appFile := components.Resource{
		Path:    fmt.Sprintf(".%v.app", name),
		Parent:  comp.Name,
		Kind:    components.ResourceTypeAppFile,
		Content: string(appfileData),
	}
	return comp.Resources.Add(appFile)
}

type ComponentInstanceStage struct{}

func (s *ComponentInstanceStage) Process(ctx context.Context, comp *components.Component) error {
	if comp.Instance == "" {
		return nil
	}
	for _, r := range comp.Resources.List() {
		comp.Resources.Delete(r.Path)
		if r.Kind == components.ResourceTypePodmanSecret {
			r.Path = comp.Instantiate(r.Path)
			comp.Resources.Set(r)
			continue
		}
		r = comp.InstantiateResource(r)
		comp.Resources.Set(r)
	}

	return nil
}
