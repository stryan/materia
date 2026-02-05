package loader

import (
	"bytes"
	"context"
	"text/template"

	"primamateria.systems/materia/internal/attributes"
	"primamateria.systems/materia/internal/macros"
	"primamateria.systems/materia/pkg/components"
)

type TemplateProcessorStage struct {
	macros macros.MacroMap
	attrs  map[string]any
}

func (s *TemplateProcessorStage) Process(ctx context.Context, comp *components.Component) error {
	vars := attributes.MergeAttributes(s.attrs, comp.Defaults)
	for _, r := range comp.Resources.List() {
		if r.Template {
			bodyTemplate := r.Content
			result := bytes.NewBuffer([]byte{})
			tmpl, err := template.New("resource").Option("missingkey=error").Funcs(s.macros(vars)).Parse(bodyTemplate)
			if err != nil {
				return err
			}
			err = tmpl.Execute(result, vars)
			if err != nil {
				return err
			}

			r.Content = result.String()
			comp.Resources.Set(r)
		}
	}
	return nil
}
