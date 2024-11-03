package materia

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"os"

	"git.saintnet.tech/stryan/materia/internal/secrets"
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

func (cur Resource) diff(newRes Resource, sm secrets.SecretsManager) ([]diffmatchpatch.Diff, error) {
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
	result := bytes.NewBuffer([]byte{})
	if newRes.Template {
		tmpl, err := template.New(newRes.Name).Parse(string(newFile))
		if err != nil {
			return diffs, err
		}
		err = tmpl.Execute(result, sm.Lookup(context.Background(), secrets.SecretFilter{}))
		if err != nil {
			return diffs, err
		}

	} else {
		result = bytes.NewBuffer(newFile)
	}
	newString = result.String()
	return dmp.DiffMain(curString, newString, false), nil
}
