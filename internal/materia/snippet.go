package materia

import (
	"text/template"

	"git.saintnet.tech/stryan/materia/internal/manifests"
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
