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

type ApplicationAction struct {
	Decan   string
	Service string
	Todo    ActionType
}

type Action struct {
	Todo    ActionType
	Payload []string
}
