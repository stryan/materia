package facts

import (
	"fmt"
	"net"
	"sort"
)

type NetworkInterfaces struct {
	Ip4 []string `json:"ip_4" toml:"ip_4"`
	Ip6 []string `json:"ip_6" toml:"ip_6"`
}

func GetInterfaceIPs() (map[string]NetworkInterfaces, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	results := make(map[string]NetworkInterfaces, len(interfaces))
	for _, i := range interfaces {
		n := NetworkInterfaces{
			Ip4: []string{},
			Ip6: []string{},
		}
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		for _, a := range addrs {
			ip, _, err := net.ParseCIDR(a.String())
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR format: %w", err)
			}
			if ip4 := ip.To4(); ip4 != nil {
				n.Ip4 = append(n.Ip4, ip.String())
			} else {
				n.Ip6 = append(n.Ip6, ip.String())
			}
		}
		sort.Strings(n.Ip4)
		sort.Strings(n.Ip6)
		results[i.Name] = n

	}
	return results, nil
}
