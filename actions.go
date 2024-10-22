package main

type Action int

const (
	ApplicationActionInstall Action = iota
	ApplicationActionRemove
	ApplicationActionStart
	ApplicationActionStop
	ApplicationActionRestart
)

type ApplicationAction struct {
	Decan   string
	Service string
	Todo    Action
}
