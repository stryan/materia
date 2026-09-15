package macros

import "text/template"

type MacroMap func(map[string]any) template.FuncMap
