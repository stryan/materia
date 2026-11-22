package actions

//go:generate stringer -type ActionType -trimprefix Action
type ActionType int

const (
	ActionUnknown ActionType = iota

	ActionInstall
	ActionRemove
	ActionUpdate

	ActionStart
	ActionStop
	ActionRestart
	ActionReload
	ActionEnable
	ActionDisable
	ActionEnsure

	ActionSetup
	ActionCleanup
	ActionMount
	ActionImport
	ActionDump

	ActionExecute
)

func (t ActionType) IsServiceAction() bool {
	return t == ActionStart || t == ActionRestart || t == ActionStop || t == ActionReload || t == ActionEnable || t == ActionDisable
}

func (t ActionType) IsResourceAction() bool {
	return t == ActionInstall || t == ActionRemove || t == ActionUpdate
}

func (t ActionType) IsHostAction() bool {
	return t == ActionSetup || t == ActionCleanup || t == ActionMount || t == ActionImport || t == ActionDump
}

func AllActionTypes() []ActionType {
	return []ActionType{
		ActionInstall,
		ActionRemove,
		ActionUpdate,
		ActionStart,
		ActionStop,
		ActionRestart,
		ActionReload,
		ActionEnable,
		ActionDisable,
		ActionEnsure,
		ActionSetup,
		ActionCleanup,
		ActionMount,
		ActionImport,
		ActionDump,
		ActionExecute,
	}
}
