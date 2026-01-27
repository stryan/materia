package main

import (
	"github.com/BurntSushi/toml"
	"primamateria.systems/materia/pkg/components"
	"primamateria.systems/materia/pkg/manifests"
)

func createManifestComponent(r components.Resource) (*components.Component, error) {
	manifestComp := components.NewComponent(r.Parent)
	resources, err := components.GetResourcesFromQuadletsFile(r.Parent, r.Content)
	if err != nil {
		return nil, err
	}
	for _, res := range resources {
		manifestComp.Resources.Set(res)
	}
	if !manifestComp.Resources.Contains(manifests.ComponentManifestFile) {
		services := []components.Resource{}
		manifest := manifests.ComponentManifest{}
		for _, r := range manifestComp.Resources.List() {
			if r.Kind == components.ResourceTypeContainer || r.Kind == components.ResourceTypePod {
				services = append(services, r)
			}
		}
		for _, s := range services {
			manifest.Services = append(manifest.Services, manifests.ServiceResourceConfig{
				Service: s.Path,
				Timeout: 60,
			})
		}
		if err := manifestComp.ApplyManifest(&manifest); err != nil {
			return nil, err
		}
		manifestData, err := toml.Marshal(manifest)
		if err != nil {
			return nil, err
		}
		manifestComp.Resources.Set(components.Resource{
			Path:     manifests.ComponentManifestFile,
			Parent:   r.Parent,
			Kind:     components.ResourceTypeManifest,
			Template: false,
			Content:  string(manifestData),
		})
	} else {
		manifest, err := manifestComp.GetManifest()
		if err != nil {
			return nil, err
		}
		if err := manifestComp.ApplyManifest(manifest); err != nil {
			return nil, err
		}
	}
	manifestComp.State = components.StateFresh
	return manifestComp, nil
}
