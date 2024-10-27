package main

type ActionType int

const (
	ActionInstallComponent ActionType = iota
	ActionRemoveComponent
	ActionStartService
	ActionStopService
	ActionRestartService
	ActionInstallResource
	ActionUpdateResource
	ActionRemoveResource
)

type Action struct {
	Todo    ActionType
	Parent  *Component
	Payload Resource
}
