package materia

import (
	"fmt"
	"strings"
)

type FactsProvider interface {
	Lookup(string) (any, error)
	GetHostname() string
	GetInterfaces() []string
}

func (m *Materia) GetFacts(host bool) string {
	var result strings.Builder
	result.WriteString("Facts\n")
	result.WriteString(fmt.Sprintf("Hostname: %v\n", m.Host.GetHostname()))
	result.WriteString("Roles: ")
	for _, r := range m.Roles {
		result.WriteString(fmt.Sprintf("%v ", r))
	}
	assigned, err := m.GetAssignedComponents()
	if err == nil {
		result.WriteString("\nAssigned Components: ")
		for _, v := range assigned {
			result.WriteString(fmt.Sprintf("%v ", v))
		}
	}
	installed, err := m.Host.ListComponentNames()
	if err == nil {
		result.WriteString("\nInstalled Components: ")
		for _, v := range installed {
			result.WriteString(fmt.Sprintf("%v ", v))
		}
	}
	result.WriteString("\nNetworks: ")
	for i, v := range m.Host.GetInterfaces() {
		result.WriteString(fmt.Sprintf("\nInterface %v: %v", i, v))
	}

	return result.String()
}
