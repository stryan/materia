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
	ResourceTypeManifest
	ResourceTypeVolumeFile
	ResourceTypeScript
	ResourceTypeComponentScript

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

func (cur Resource) diff(fmap MacroMap, newRes Resource, vars map[string]interface{}) ([]diffmatchpatch.Diff, error) {
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
	var newString string
	result, err := newRes.execute(fmap, vars)
	if err != nil {
		return diffs, err
	}
	newString = result.String()
	return dmp.DiffMain(curString, newString, false), nil
}

func (cur Resource) execute(funcMap MacroMap, vars map[string]any) (*bytes.Buffer, error) {
	newFile, err := os.ReadFile(cur.Path)
	if err != nil {
		return nil, err
	}
	if !cur.Template {
		return bytes.NewBuffer(newFile), nil
	}

	result := bytes.NewBuffer([]byte{})
	tmpl, err := template.New(cur.Name).Option("missingkey=error").Funcs(funcMap(vars)).Parse(string(newFile))
	if err != nil {
		return nil, err
	}
	err = tmpl.Execute(result, vars)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// func (r Resource) getServiceFromResource() (Resource, error) {
// 	var res Resource
// 	switch r.Kind {
// 	case ResourceTypeContainer:
// 		servicename, found := strings.CutSuffix(r.Name, ".container")
// 		if !found {
// 			return res, fmt.Errorf("invalid container name for service: %v", r.Name)
// 		}
// 		res = Resource{
// 			Name: fmt.Sprintf("%v.service", servicename),
// 			Kind: ResourceTypeService,
// 		}
// 	case ResourceTypePod:
// 		podname, found := strings.CutSuffix(r.Name, ".pod")
// 		if !found {
// 			return res, fmt.Errorf("invalid pod name %v", r.Name)
// 		}
// 		res = Resource{
// 			Name: fmt.Sprintf("%v-pod.service", podname),
// 			Kind: ResourceTypeService,
// 		}
// 	case ResourceTypeService:
// 		return r, nil
// 	default:
// 		return res, errors.New("tried to convert a non container or pod to a service")
// 	}
// 	return res, nil
// }

func (r Resource) toAction(action string) ActionType {
	todo := ActionUnknown
	switch action {
	case "install":
		switch r.Kind {
		case ResourceTypeContainer, ResourceTypeKube, ResourceTypeNetwork, ResourceTypePod, ResourceTypeVolume:
			todo = ActionInstallQuadlet
		case ResourceTypeFile, ResourceTypeManifest:
			todo = ActionInstallFile
		case ResourceTypeComponentScript:
			todo = ActionInstallComponentScript
		case ResourceTypeScript:
			todo = ActionInstallScript
		case ResourceTypeService:
			todo = ActionInstallService
		case ResourceTypeVolumeFile:
			todo = ActionInstallVolumeFile

		}
	case "update":
		switch r.Kind {
		case ResourceTypeContainer, ResourceTypeKube, ResourceTypeNetwork, ResourceTypePod, ResourceTypeVolume:
			todo = ActionUpdateQuadlet
		case ResourceTypeFile, ResourceTypeManifest:
			todo = ActionUpdateFile
		case ResourceTypeScript:
			todo = ActionUpdateScript
		case ResourceTypeService:
			todo = ActionUpdateService
		case ResourceTypeVolumeFile:
			todo = ActionUpdateVolumeFile
		case ResourceTypeComponentScript:
			todo = ActionUpdateComponentScript
		}
	case "remove":
		switch r.Kind {
		case ResourceTypeContainer, ResourceTypeKube, ResourceTypeNetwork, ResourceTypePod, ResourceTypeVolume:
			todo = ActionRemoveQuadlet
		case ResourceTypeFile, ResourceTypeManifest:
			todo = ActionRemoveFile
		case ResourceTypeScript:
			todo = ActionRemoveScript
		case ResourceTypeService:
			todo = ActionRemoveService
		case ResourceTypeVolumeFile:
			todo = ActionRemoveVolumeFile
		case ResourceTypeComponentScript:
			todo = ActionRemoveComponentScript
		}
	}
	return todo
}
