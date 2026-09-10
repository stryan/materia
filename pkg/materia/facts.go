package materia

import (
	"fmt"
)

type FactsProvider interface {
	Lookup(string) (any, error)
	GetHostname() string
	GetInterfaces() []string
}

func (m *Materia) GetFacts(host bool) string {
	var result string
	result += "Facts\n"
	result += fmt.Sprintf("Hostname: %v\n", m.Host.GetHostname())
	result += "Roles: "
	for _, r := range m.Roles {
		result += fmt.Sprintf("%v ", r)
	}
	assigned, err := m.GetAssignedComponents()
	if err == nil {
		result += "\nAssigned Components: "
		for _, v := range assigned {
			result += fmt.Sprintf("%v ", v)
		}
	}
	installed, err := m.Host.ListComponentNames()
	if err == nil {
		result += "\nInstalled Components: "
		for _, v := range installed {
			result += fmt.Sprintf("%v ", v)
		}
	}
	result += "\nNetworks: "
	for i, v := range m.Host.GetInterfaces() {
		result += fmt.Sprintf("\nInterface %v: %v", i, v)
	}

	return result
}
