package materia

import "fmt"

type FactsProvider interface {
	Lookup(string) (any, error)
	GetHostname() string
	GetInterfaces() []string
}

func (m *Materia) GetFacts(host bool) string {
	var result string
	result += "Facts\n"
	result += fmt.Sprintf("Hostname: %v\n", m.HostFacts.GetHostname())
	result += "Roles: "
	for _, r := range m.Roles {
		result += fmt.Sprintf("%v ", r)
	}
	result += "\nAssigned Components: "
	for _, v := range m.AssignedComponents {
		result += fmt.Sprintf("%v ", v)
	}
	result += "\nInstalled Components: "
	for _, v := range m.InstalledComponents {
		result += fmt.Sprintf("%v ", v)
	}
	result += "\nNetworks: "
	for i, v := range m.HostFacts.GetInterfaces() {
		result += fmt.Sprintf("\nInterface %v: %v", i, v)
	}

	return result
}
