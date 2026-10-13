package facts

import (
	"context"
	"fmt"
	"os"
)

const factsNamespace = "host"

type HostCollector struct{}

type HostFacts struct {
	Hostname   string                       `json:"hostname" toml:"hostname"`
	Interfaces map[string]NetworkInterfaces `json:"interfaces" toml:"interfaces"`
}

func (h *HostCollector) Namespace() string {
	return "host"
}

func (h *HostCollector) Collect(_ context.Context, _ *Facts) (any, error) {
	var err error
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("error getting hostname: %w", err)
	}
	networks, err := GetInterfaceIPs()
	if err != nil {
		return nil, fmt.Errorf("error getting network interfaces: %w", err)
	}
	return HostFacts{
		Hostname:   hostname,
		Interfaces: networks,
	}, nil
}

func ToHostFacts(f *Facts) (HostFacts, error) {
	return GetTypedFacts[HostFacts](f, factsNamespace)
}

func ResolveHostname(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return os.Hostname()
}
