package materia

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"text/template"

	"primamateria.systems/materia/pkg/executor"
	"primamateria.systems/materia/pkg/facts"
	"primamateria.systems/materia/pkg/macros"
	"primamateria.systems/materia/pkg/manifests"
)

func configToSnippet(c manifests.SnippetConfig) (*macros.Snippet, error) {
	var err error
	t := template.New(c.Name)
	t, err = t.Parse(c.Body)
	if err != nil {
		return nil, err
	}
	return &macros.Snippet{
		Name:       c.Name,
		Parameters: c.Parameters,
		Body:       t,
	}, nil
}

func loadDefaultSnippets() []*macros.Snippet {
	return []*macros.Snippet{
		{
			Name: "onBoot",
			Body: template.Must(template.New("onBoot").Parse("[Install]\nWantedBy=default.target")),
		},
		{
			Name: "harden",
			Body: template.Must(template.New("harden").Parse("DropCapability=ALL\nReadOnly=true\n\nNoNewPrivileges=true")),
		},
	}
}

// TODO refactor out MateriaConfig, Hostmanager and replace with actual important structs (Containers Manager and more Facts)
func loadDefaultMacros(c *MateriaConfig, ec executor.ExecutorConfig, host HostManager, f *facts.Facts, snippets map[string]*macros.Snippet) macros.MacroMap {
	return func(vars map[string]any) template.FuncMap {
		return template.FuncMap{
			"m_dataDir": func(arg string) (string, error) {
				return filepath.Join(filepath.Join(ec.MateriaDir, "components"), arg), nil
			},
			"m_quadletDir": func(arg string) (string, error) {
				return filepath.Join(filepath.Join(ec.QuadletDir, "components"), arg), nil
			},
			"m_outputDir": func(arg string) (string, error) {
				return ec.OutputDir, nil
			},
			"m_scriptsDir": func(_ string) (string, error) {
				return ec.ScriptsDir, nil
			},
			"m_serviceDir": func(_ string) (string, error) {
				return ec.ServiceDir, nil
			},
			"m_facts": func(arg string) (any, error) {
				return f.LookupString(arg)
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
			"isRoot": func(_ string) bool {
				return c.User.Username == "root" || !c.Rootless
			},
			"secretEnv": func(args ...string) string {
				if len(args) == 0 {
					return ""
				}
				if len(args) == 1 {
					return fmt.Sprintf("Secret=%v,type=env,target=%v", host.SecretName(args[0]), args[0])
				}
				return fmt.Sprintf("Secret=%v,type=env,target=%v", host.SecretName(args[0]), args[1])
			},
			"secretMount": func(args ...string) string {
				if len(args) == 0 {
					return ""
				}
				if len(args) == 1 {
					return fmt.Sprintf("Secret=%v,type=mount,target=%v", host.SecretName(args[0]), args[0])
				}
				return fmt.Sprintf("Secret=%v,type=mount,target=%v", host.SecretName(args[0]), args[1])
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
