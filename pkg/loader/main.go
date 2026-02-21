package loader

import (
	"context"

	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/internal/macros"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

type ComponentLoadPipeline struct {
	stages []ComponentLoadStage
}

type ComponentLoadStage interface {
	Process(ctx context.Context, comp *components.Component) error
}

func (p *ComponentLoadPipeline) AddStage(stage ComponentLoadStage) error {
	p.stages = append(p.stages, stage)
	return nil
}

func NewHostComponentPipeline(mgr components.ComponentReader, cont containers.ContainerManager) *ComponentLoadPipeline {
	return &ComponentLoadPipeline{
		stages: []ComponentLoadStage{
			&ComponentInitStage{manager: mgr},
			&ManifestLoadStage{manager: mgr},
			&ResourceDiscoveryStage{manager: mgr},
			&QuadletObjectExtractorStage{},
			&SecretDiscoveryStage{manager: cont},
			&StateInjectorStage{components.StateStale},
		},
	}
}

func NewSourceComponentPipeline(mgr components.ComponentReader, macros macros.MacroMap, attrs map[string]any, overrides, extensions []*manifests.ComponentManifest) *ComponentLoadPipeline {
	return &ComponentLoadPipeline{
		stages: []ComponentLoadStage{
			&ComponentInitStage{manager: mgr},
			&ManifestLoadStage{
				manager:    mgr,
				overrides:  overrides,
				extensions: extensions,
			},
			&ResourceDiscoveryStage{manager: mgr},
			&TemplateProcessorStage{macros: macros, attrs: attrs},
			&SecretInjectorStage{attrs: attrs},
			&QuadletExpanderStage{},
			&QuadletObjectExtractorStage{},
			&StateInjectorStage{components.StateFresh},
		},
	}
}

func (p *ComponentLoadPipeline) Load(ctx context.Context, comp *components.Component) error {
	for _, stage := range p.stages {
		if err := stage.Process(ctx, comp); err != nil {
			return err
		}
	}
	return comp.Validate()
}
