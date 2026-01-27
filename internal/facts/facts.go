package facts

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type HostFactsManager struct {
	Hostname   string
	Interfaces map[string]NetworkInterfaces
}

func NewHostFacts(hostname string) (*HostFactsManager, error) {
	facts := &HostFactsManager{}
	var err error
	if hostname != "" {
		facts.Hostname = hostname
	} else {
		facts.Hostname, err = os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("error getting hostname: %w", err)
		}
	}
	networks, err := GetInterfaceIPs()
	if err != nil {
		return nil, fmt.Errorf("error getting network interfaces: %w", err)
	}
	facts.Interfaces = networks
	return facts, nil
}

func (f *HostFactsManager) Lookup(arg string) (any, error) {
	input := strings.Split(arg, ".")
	switch input[0] {
	case "hostname":
		return f.Hostname, nil
	case "interface":
		if len(input) == 1 {
			return f.Interfaces, nil
		}
		if len(input) == 2 {
			return f.Interfaces[input[1]], nil
		}
		if len(input) == 3 {
			if input[2] == "ip4" {
				return f.Interfaces[input[1]].Ip4, nil
			}
			if input[2] == "ip6" {
				return f.Interfaces[input[1]].Ip6, nil
			}
			return nil, errors.New("invalid ip type")
		}
		if len(input) == 4 {
			index, err := strconv.Atoi(input[3])
			if err != nil {
				return nil, errors.New("invalid interface index")
			}
			if input[2] == "ip4" {
				return f.Interfaces[input[1]].Ip4[index], nil
			}
			if input[2] == "ip6" {
				return f.Interfaces[input[1]].Ip6[index], nil
			}
		}
	}
	return nil, errors.New("invalid fact lookup")
}

func (f *HostFactsManager) GetHostname() string {
	return f.Hostname
}

func (f *HostFactsManager) GetInterfaces() []string {
	var result []string
	for k := range f.Interfaces {
		result = append(result, k)
	}
	return result
}
