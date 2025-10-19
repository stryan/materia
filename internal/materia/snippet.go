package materia

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"text/template"

	"primamateria.systems/materia/internal/manifests"
)

type Snippet struct {
	Name       string
	Parameters []string
	Body       *template.Template
}

func configToSnippet(c manifests.SnippetConfig) (*Snippet, error) {
	var err error
	t := template.New(c.Name)
	t, err = t.Parse(c.Body)
	if err != nil {
		return nil, err
	}
	return &Snippet{
		Name:       c.Name,
		Parameters: c.Parameters,
		Body:       t,
	}, nil
}

func loadDefaultSnippets() []*Snippet {
	return []*Snippet{
		{
			Name:       "autoUpdate",
			Parameters: []string{"source"},
			Body:       template.Must(template.New("autoUpdate").Parse("Label=io.containers.autoupdate={{ .source }}")),
		},
	}
}

func loadDefaultMacros(c *MateriaConfig, containers ContainerManager, facts FactsProvider, snippets map[string]*Snippet) MacroMap {
	return func(vars map[string]any) template.FuncMap {
		return template.FuncMap{
			"m_dataDir": func(arg string) (string, error) {
				return filepath.Join(filepath.Join(c.MateriaDir, "materia", "components"), arg), nil
			},
			"m_facts": func(arg string) (any, error) {
				return facts.Lookup(arg)
			},
			"m_default": func(arg string, def string) string {
				val, ok := vars[arg]
				if ok {
					return val.(string)
				}
				return def
			},
			"exists": func(arg string) bool {
				_, ok := vars[arg]
				return ok
			},
			"secretEnv": func(args ...string) string {
				if len(args) == 0 {
					return ""
				}
				if len(args) == 1 {
					return fmt.Sprintf("Secret=%v,type=env,target=%v", containers.SecretName(args[0]), args[0])
				}
				return fmt.Sprintf("Secret=%v,type=env,target=%v", containers.SecretName(args[0]), args[1])
			},
			"secretMount": func(args ...string) string {
				if len(args) == 0 {
					return ""
				}
				if len(args) == 1 {
					return fmt.Sprintf("Secret=%v,type=mount,target=%v", containers.SecretName(args[0]), args[0])
				}
				return fmt.Sprintf("Secret=%v,type=env,%s", containers.SecretName(args[0]), args[1])
			},
			"snippet": func(name string, args ...string) (string, error) {
				s, ok := snippets[name]
				if !ok {
					return "", errors.New("snippet not found")
				}
				snipVars := make(map[string]string, len(s.Parameters))
				for k, v := range s.Parameters {
					snipVars[v] = args[k]
				}

				result := bytes.NewBuffer([]byte{})
				err := s.Body.Execute(result, snipVars)
				return result.String(), err
			},
		}
	}
}
