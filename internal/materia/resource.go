package materia

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"os"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type Resource struct {
	Path     string
	Name     string
	Kind     ResourceType
	Template bool
}

//go:generate stringer -type ResourceType -trimprefix ResourceType
type ResourceType uint

const (
	ResourceTypeUnknown ResourceType = iota
	ResourceTypeContainer
	ResourceTypeVolume
	ResourceTypePod
	ResourceTypeNetwork
	ResourceTypeKube
	ResourceTypeFile
	ResourceTypeVolumeFile

	// special types that exist after systemctl daemon-reload
	ResourceTypeService
)

func (r Resource) Validate() error {
	if r.Kind == ResourceTypeUnknown {
		return errors.New("unknown resource type")
	}
	if r.Name == "" {
		return errors.New("resource without name")
	}
	return nil
}

func (r *Resource) String() string {
	return fmt.Sprintf("{r %v %v %v %v }", r.Name, r.Path, r.Kind, r.Template)
}

func (cur Resource) diff(newRes Resource, vars map[string]interface{}) ([]diffmatchpatch.Diff, error) {
	dmp := diffmatchpatch.New()
	var diffs []diffmatchpatch.Diff
	if err := cur.Validate(); err != nil {
		return diffs, fmt.Errorf("self resource invalid during comparison: %w", err)
	}
	if err := newRes.Validate(); err != nil {
		return diffs, fmt.Errorf("other resource invalid during comparison: %w", err)
	}
	curFile, err := os.ReadFile(cur.Path)
	if err != nil {
		return diffs, err
	}
	curString := string(curFile)
	// parse if template
	newFile, err := os.ReadFile(newRes.Path)
	if err != nil {
		return diffs, err
	}
	var newString string
	var result *bytes.Buffer
	if newRes.Template {
		result, err = newRes.execute(vars)
		if err != nil {
			return diffs, err
		}

	} else {
		result = bytes.NewBuffer(newFile)
	}
	newString = result.String()
	return dmp.DiffMain(curString, newString, false), nil
}

func (cur Resource) execute(vars map[string]interface{}) (*bytes.Buffer, error) {
	newFile, err := os.ReadFile(cur.Path)
	if err != nil {
		return nil, err
	}

	result := bytes.NewBuffer([]byte{})
	tmpl, err := template.New(cur.Name).Option("missingkey=error").Funcs(template.FuncMap{
		"materia_defaults": func(arg string) string {
			switch arg {
			case "after":
				if res, ok := vars["After"]; ok {
					return res.(string)
				} else {
					return "local-fs.target network.target"
				}
			case "wants":
				if res, ok := vars["Wants"]; ok {
					return res.(string)
				} else {
					return "local-fs.target network.target"
				}
			case "requires":
				if res, ok := vars["Requires"]; ok {
					return res.(string)
				} else {
					return "local-fs.target network.target"
				}
			default:
				return "ERR_BAD_DEFAULT"
			}
		},
		"exists": func(arg string) bool {
			_, ok := vars[arg]
			return ok
		},
	}).Parse(string(newFile))
	if err != nil {
		return nil, err
	}
	err = tmpl.Execute(result, vars)
	if err != nil {
		return nil, err
	}
	return result, nil
}
