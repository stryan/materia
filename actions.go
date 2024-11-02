package main

type ActionType int

const (
	ActionInstallComponent ActionType = iota
	ActionRemoveComponent
	ActionReloadUnits
	ActionStartService
	ActionStopService
	ActionRestartService
	ActionInstallResource
	ActionInstallVolumeResource
	ActionUpdateResource
	ActionRemoveResource
)

type Action struct {
	Todo    ActionType
	Parent  *Component
	Payload Resource
}
