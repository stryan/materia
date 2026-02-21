package executor

import (
	"context"

	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/pkg/components"
)

type actionHandler func(context.Context, *Executor, actions.Action) error

var handlerList = map[components.ResourceType]map[actions.ActionType]actionHandler{
	components.ResourceTypeComponent: {
		actions.ActionInstall: installComponent,
		actions.ActionUpdate:  updateComponent,
		actions.ActionRemove:  removeComponent,
	},
	components.ResourceTypeVolume: {
		actions.ActionInstall: installOrUpdateFile,
		actions.ActionUpdate:  installOrUpdateFile,
		actions.ActionRemove:  removeFile,
		actions.ActionEnsure:  ensureQuadlet,
		actions.ActionCleanup: cleanupVolume,
		actions.ActionImport:  importVolume,
		actions.ActionDump:    dumpVolume,
	},
	components.ResourceTypeContainer: {
		actions.ActionInstall: installOrUpdateFile,
		actions.ActionUpdate:  installOrUpdateFile,
		actions.ActionRemove:  removeFile,
		actions.ActionEnsure:  ensureQuadlet,
		actions.ActionStart:   serviceAction,
		actions.ActionStop:    serviceAction,
		actions.ActionRestart: serviceAction,
		actions.ActionEnable:  serviceAction,
		actions.ActionDisable: serviceAction,
	},
	components.ResourceTypePod: {
		actions.ActionInstall: installOrUpdateFile,
		actions.ActionUpdate:  installOrUpdateFile,
		actions.ActionRemove:  removeFile,
		actions.ActionEnsure:  ensureQuadlet,
		actions.ActionStart:   serviceAction,
		actions.ActionStop:    serviceAction,
		actions.ActionRestart: serviceAction,
		actions.ActionEnable:  serviceAction,
		actions.ActionDisable: serviceAction,
	},
	components.ResourceTypeNetwork: {
		actions.ActionInstall: installOrUpdateFile,
		actions.ActionUpdate:  installOrUpdateFile,
		actions.ActionRemove:  removeFile,
		actions.ActionEnsure:  ensureQuadlet,
		actions.ActionStart:   serviceAction,
		actions.ActionStop:    serviceAction,
		actions.ActionRestart: serviceAction,
		actions.ActionEnable:  serviceAction,
		actions.ActionDisable: serviceAction,
		actions.ActionCleanup: cleanupNetwork,
	},
	components.ResourceTypeKube: {
		actions.ActionInstall: installOrUpdateFile,
		actions.ActionUpdate:  installOrUpdateFile,
		actions.ActionRemove:  removeFile,
		actions.ActionEnsure:  ensureQuadlet,
		actions.ActionStart:   serviceAction,
		actions.ActionStop:    serviceAction,
		actions.ActionRestart: serviceAction,
		actions.ActionEnable:  serviceAction,
		actions.ActionDisable: serviceAction,
	},
	components.ResourceTypeBuild: {
		actions.ActionInstall: installOrUpdateFile,
		actions.ActionUpdate:  installOrUpdateFile,
		actions.ActionRemove:  removeFile,
		actions.ActionEnsure:  ensureQuadlet,
		actions.ActionStart:   serviceAction,
		actions.ActionStop:    serviceAction,
		actions.ActionRestart: serviceAction,
		actions.ActionEnable:  serviceAction,
		actions.ActionDisable: serviceAction,
		actions.ActionCleanup: cleanupBuildArtifact,
	},
	components.ResourceTypeImage: {
		actions.ActionInstall: installOrUpdateFile,
		actions.ActionUpdate:  installOrUpdateFile,
		actions.ActionRemove:  removeFile,
		actions.ActionEnsure:  ensureQuadlet,
		actions.ActionStart:   serviceAction,
		actions.ActionStop:    serviceAction,
		actions.ActionRestart: serviceAction,
		actions.ActionEnable:  serviceAction,
		actions.ActionDisable: serviceAction,
		actions.ActionCleanup: cleanupBuildArtifact,
	},
	components.ResourceTypeAppFile: {
		actions.ActionInstall: installOrUpdateFile,
		actions.ActionUpdate:  installOrUpdateFile,
		actions.ActionRemove:  removeFile,
	},
	components.ResourceTypeFile: {
		actions.ActionInstall: installOrUpdateFile,
		actions.ActionUpdate:  installOrUpdateFile,
		actions.ActionRemove:  removeFile,
	},
	components.ResourceTypeManifest: {
		actions.ActionInstall: installOrUpdateFile,
		actions.ActionUpdate:  installOrUpdateFile,
		actions.ActionRemove:  removeFile,
	},
	components.ResourceTypeScript: {
		actions.ActionInstall: installOrUpdateScript,
		actions.ActionUpdate:  installOrUpdateScript,
		actions.ActionRemove:  removeScript,
		actions.ActionSetup:   setupScript,
		actions.ActionCleanup: cleanupScript,
	},
	components.ResourceTypeDirectory: {
		actions.ActionInstall: installDir,
		actions.ActionRemove:  removeDir,
	},
	components.ResourceTypeHost: {
		actions.ActionReload: serviceAction,
	},
	components.ResourceTypeService: {
		actions.ActionInstall: installOrUpdateUnit,
		actions.ActionUpdate:  installOrUpdateUnit,
		actions.ActionRemove:  removeUnit,
		actions.ActionStart:   serviceAction,
		actions.ActionStop:    serviceAction,
		actions.ActionRestart: serviceAction,
		actions.ActionEnable:  serviceAction,
		actions.ActionDisable: serviceAction,
	},
	components.ResourceTypePodmanSecret: {
		actions.ActionInstall: installOrUpdateSecret,
		actions.ActionUpdate:  installOrUpdateSecret,
		actions.ActionRemove:  removeSecret,
	},
}
