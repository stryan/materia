package services

type ServiceState string

// active, reloading, inactive, failed, activating, deactivating
const (
	StateUnknown      ServiceState = "unknown"
	StateActive       ServiceState = "active"
	StateReloading    ServiceState = "reloading"
	StateInactive     ServiceState = "inactive"
	StateFailed       ServiceState = "failed"
	StateActivating   ServiceState = "activating"
	StateDeactivating ServiceState = "deactivating"
	StateMaintenance  ServiceState = "maintenance"
	StateRefreshing   ServiceState = "refreshing"

	StateInternalWildcard ServiceState = "wildcard" // Note we don't put this in the map since its internal
)

var serviceStateMap = map[string]ServiceState{
	"unknown":      StateUnknown,
	"active":       StateActive,
	"reloading":    StateReloading,
	"inactive":     StateInactive,
	"failed":       StateFailed,
	"activating":   StateActivating,
	"deactivating": StateDeactivating,
	"maintenance":  StateMaintenance,
	"refreshing":   StateRefreshing,
}

func NewServiceState(name string) ServiceState {
	if res, ok := serviceStateMap[name]; !ok {
		return StateUnknown
	} else {
		return res
	}
}

type ServiceEnableState string

const (
	EnableStateUnknown        ServiceEnableState = "unknown"
	EnableStateEnabled        ServiceEnableState = "enabled"
	EnableStateEnabledRuntime ServiceEnableState = "enabled-runtime"
	EnableStateLinked         ServiceEnableState = "linked"
	EnableStateLinkedRuntime  ServiceEnableState = "linked-runtime"
	EnableStateAlias          ServiceEnableState = "alias"
	EnableStateMasked         ServiceEnableState = "masked"
	EnableStateMaskedRuntime  ServiceEnableState = "masked-runtime"
	EnableStateStatic         ServiceEnableState = "static"
	EnableStateIndirect       ServiceEnableState = "indirect"
	EnableStateDisabled       ServiceEnableState = "disabled"
	EnableStateGenerated      ServiceEnableState = "generated"
	EnableStateTransient      ServiceEnableState = "transient"
	EnableStateBad            ServiceEnableState = "bad"
	EnableStateNotFound       ServiceEnableState = "not-found"

	EnableStateInternalWildcard ServiceEnableState = "wildcard" // Note we don't put this in the map since its internal
)

var serviceEnableStateMap = map[string]ServiceEnableState{
	"unknown":         EnableStateUnknown,
	"enabled":         EnableStateEnabled,
	"enabled-runtime": EnableStateEnabled,
	"linked":          EnableStateLinked,
	"linked-runtime":  EnableStateLinkedRuntime,
	"alias":           EnableStateAlias,
	"masked":          EnableStateMasked,
	"masked-runtime":  EnableStateMaskedRuntime,
	"static":          EnableStateStatic,
	"indirect":        EnableStateIndirect,
	"disabled":        EnableStateDisabled,
	"generated":       EnableStateGenerated,
	"transient":       EnableStateTransient,
	"bad":             EnableStateBad,
}

func NewServiceEnableState(name string) ServiceEnableState {
	if res, ok := serviceEnableStateMap[name]; !ok {
		return EnableStateUnknown
	} else {
		return res
	}
}
