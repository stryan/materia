package macros

import "text/template"

type Snippet struct {
	Name       string
	Parameters []string
	Body       *template.Template
}
