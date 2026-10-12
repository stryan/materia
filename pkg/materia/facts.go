package materia

type FactsProvider interface {
	Lookup(string) (any, error)
	GetHostname() string
	GetInterfaces() []string
}
