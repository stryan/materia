package materia

import (
	"cmp"
	"slices"

	"git.saintnet.tech/stryan/materia/internal/components"
)

func sortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	keys := make([]K, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	slices.Sort(keys)
	return keys
}

func resToAction(r components.Resource, action string) ActionType {
	todo := ActionUnknown
	switch action {
	case "install":
		switch r.Kind {
		case components.ResourceTypeContainer, components.ResourceTypeKube, components.ResourceTypeNetwork, components.ResourceTypePod, components.ResourceTypeVolume:
			todo = ActionInstallQuadlet
		case components.ResourceTypeFile, components.ResourceTypeManifest:
			todo = ActionInstallFile
		case components.ResourceTypeComponentScript:
			todo = ActionInstallComponentScript
		case components.ResourceTypeScript:
			todo = ActionInstallScript
		case components.ResourceTypeService:
			todo = ActionInstallService
		case components.ResourceTypeVolumeFile:
			todo = ActionInstallVolumeFile

		}
	case "update":
		switch r.Kind {
		case components.ResourceTypeContainer, components.ResourceTypeKube, components.ResourceTypeNetwork, components.ResourceTypePod, components.ResourceTypeVolume:
			todo = ActionUpdateQuadlet
		case components.ResourceTypeFile, components.ResourceTypeManifest:
			todo = ActionUpdateFile
		case components.ResourceTypeScript:
			todo = ActionUpdateScript
		case components.ResourceTypeService:
			todo = ActionUpdateService
		case components.ResourceTypeVolumeFile:
			todo = ActionUpdateVolumeFile
		case components.ResourceTypeComponentScript:
			todo = ActionUpdateComponentScript
		}
	case "remove":
		switch r.Kind {
		case components.ResourceTypeContainer, components.ResourceTypeKube, components.ResourceTypeNetwork, components.ResourceTypePod, components.ResourceTypeVolume:
			todo = ActionRemoveQuadlet
		case components.ResourceTypeFile, components.ResourceTypeManifest:
			todo = ActionRemoveFile
		case components.ResourceTypeScript:
			todo = ActionRemoveScript
		case components.ResourceTypeService:
			todo = ActionRemoveService
		case components.ResourceTypeVolumeFile:
			todo = ActionRemoveVolumeFile
		case components.ResourceTypeComponentScript:
			todo = ActionRemoveComponentScript
		}
	}
	return todo
}
