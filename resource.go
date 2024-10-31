package main

import (
	"bytes"
	"context"
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

type ResourceType int

const (
	ResourceTypeContainer ResourceType = iota
	ResourceTypeVolume
	ResourceTypePod
	ResourceTypeNetwork
	ResourceTypeKube
	ResourceTypeFile
	ResourceTypeVolumeFile

	// special types that exist after systemctl daemon-reload
	ResourceTypeService
)

func (cur Resource) Diff(newRes Resource, sm secrets.SecretsManager) ([]diffmatchpatch.Diff, error) {
	dmp := diffmatchpatch.New()
	var diffs []diffmatchpatch.Diff
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
