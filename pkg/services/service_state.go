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

type ServiceEnableStatue string

const (
	EnableStateUnknown        ServiceEnableStatue = "unknown"
	EnableStateEnabled        ServiceEnableStatue = "enabled"
	EnableStateEnabledRuntime ServiceEnableStatue = "enabled-runtime"
	EnableStateLinked         ServiceEnableStatue = "linked"
	EnableStateLinkedRuntime  ServiceEnableStatue = "linked-runtime"
	EnableStateAlias          ServiceEnableStatue = "alias"
	EnableStateMasked         ServiceEnableStatue = "masked"
	EnableStateMaskedRuntime  ServiceEnableStatue = "masked-runtime"
	EnableStateStatic         ServiceEnableStatue = "static"
	EnableStateIndirect       ServiceEnableStatue = "indirect"
	EnableStateDisabled       ServiceEnableStatue = "disabled"
	EnableStateGenerated      ServiceEnableStatue = "generated"
	EnableStateTransient      ServiceEnableStatue = "transient"
	EnableStateBad            ServiceEnableStatue = "bad"
	EnableStateNotFound       ServiceEnableStatue = "not-found"
)

var serviceEnableStateMap = map[string]ServiceEnableStatue{
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

func NewServiceEnableState(name string) ServiceEnableStatue {
	if res, ok := serviceEnableStateMap[name]; !ok {
		return EnableStateUnknown
	} else {
		return res
	}
}
