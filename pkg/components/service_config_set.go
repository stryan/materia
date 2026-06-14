package components

import (
	"cmp"
	"errors"

	"github.com/emirpasic/gods/sets"
	"github.com/emirpasic/gods/sets/treeset"
	"primamateria.systems/materia/pkg/manifests"
)

var ErrServiceConfigNotFound = errors.New("service config not found")

type ServiceConfigSet struct {
	sets.Set
}

func compareServiceResource(left, right interface{}) int {
	leftSRC := left.(manifests.ServiceResourceConfig)
	rightSRC := right.(manifests.ServiceResourceConfig)
	return cmp.Compare(leftSRC.Service, rightSRC.Service)
}

func NewServiceConfigSet() *ServiceConfigSet {
	return &ServiceConfigSet{
		treeset.NewWith(compareServiceResource),
	}
}

func (s *ServiceConfigSet) List() []manifests.ServiceResourceConfig {
	result := make([]manifests.ServiceResourceConfig, s.Size())
	for k, v := range s.Values() {
		result[k] = v.(manifests.ServiceResourceConfig)
	}
	return result
}

func (s *ServiceConfigSet) ListServiceNames() []string {
	result := make([]string, s.Size())
	for k, v := range s.Values() {
		result[k] = v.(manifests.ServiceResourceConfig).Service
	}
	return result
}

func (s *ServiceConfigSet) Get(name string) (manifests.ServiceResourceConfig, error) {
	for _, v := range s.Values() {
		src := v.(manifests.ServiceResourceConfig)
		if src.Service == name {
			return src, nil
		}
	}
	return manifests.ServiceResourceConfig{}, ErrServiceConfigNotFound
}
