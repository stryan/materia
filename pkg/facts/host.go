package facts

import (
	"context"
	"fmt"
	"os"
)

type HostCollector struct {
	hostname string
}

type HostFacts struct {
	Hostname   string
	Interfaces map[string]NetworkInterfaces
}

func (h *HostCollector) Namespace() string {
	return "host"
}

func (h *HostCollector) Collect(_ context.Context, _ *Facts) (any, error) {
	hostname := h.hostname
	var err error
	if hostname == "" {
		hostname, err = os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("error getting hostname: %w", err)
		}
	}
	networks, err := GetInterfaceIPs()
	if err != nil {
		return nil, fmt.Errorf("error getting network interfaces: %w", err)
	}
	return &HostFacts{
		Hostname:   hostname,
		Interfaces: networks,
	}, nil
}
