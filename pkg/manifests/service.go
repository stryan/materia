package manifests

import (
	"errors"
	"fmt"
)

type ServiceType string

type ServiceState string

const (
	ServiceTypeQuadlet ServiceType = "quadlet"
	ServiceTypeUnit    ServiceType = "unit"

	ServiceStateNormal ServiceState = "normal"
	ServiceStateLazy   ServiceState = "lazy"
	ServiceStateInert  ServiceState = "inert"
)

type ServiceManifest struct {
	Service string `toml:"Service"`

	Type  ServiceType  `toml:"Type"`
	State ServiceState `toml:"State"`

	Oneshot  bool `toml:"Oneshot"`
	Disabled bool `toml:"Disabled"`

	RestartedBy []string `toml:"RestartedBy"`
	ReloadedBy  []string `toml:"ReloadedBy"`
	Timeout     int      `toml:"Timeout"`

	// Deprecated: use Type = "unit".
	Static bool `toml:"Static,omitempty"`
	// Deprecated: use State = "lazy".
	Stopped bool `toml:"Stopped,omitempty"`
}

func (sm ServiceManifest) Validate() error {
	if sm.Service == "" {
		return errors.New("service manifest without a name")
	}
	switch sm.Type {
	case "", ServiceTypeQuadlet, ServiceTypeUnit:
	default:
		return fmt.Errorf("service %q has unknown type %q", sm.Service, sm.Type)
	}
	switch sm.State {
	case "", ServiceStateNormal, ServiceStateLazy, ServiceStateInert:
	default:
		return fmt.Errorf("service %q has unknown state %q", sm.Service, sm.State)
	}
	return nil
}

func (sm ServiceManifest) state() ServiceState {
	if sm.State == "" {
		return ServiceStateNormal
	}
	return sm.State
}

func (sm ServiceManifest) CanEnable() bool {
	return sm.Type == ServiceTypeUnit && sm.state() != ServiceStateInert && !sm.Disabled
}

func (sm ServiceManifest) CanStart() bool {
	return sm.state() == ServiceStateNormal
}

func (sm ServiceManifest) CanTrigger() bool {
	return sm.state() == ServiceStateNormal || sm.state() == ServiceStateLazy
}

func (sm ServiceManifest) IsOneshot() bool {
	return sm.Oneshot
}

func DefaultQuadletManifest(name string) ServiceManifest {
	return ServiceManifest{
		Service: name,
		Type:    ServiceTypeQuadlet,
		State:   ServiceStateNormal,
	}
}

func DefaultUnitManifest(name string) ServiceManifest {
	return ServiceManifest{
		Service: name,
		Type:    ServiceTypeUnit,
		State:   ServiceStateNormal,
	}
}

func (sm ServiceManifest) Migrate() ServiceManifest {
	if sm.Static {
		sm.Type = ServiceTypeUnit
		sm.Static = false
	}
	if sm.Stopped {
		sm.State = ServiceStateLazy
		sm.Stopped = false
	}
	return sm
}

func (sm ServiceManifest) Deprecated() bool {
	return sm.Static || sm.Stopped
}
