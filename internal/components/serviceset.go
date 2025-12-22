package components

import (
	"cmp"
	"errors"

	"github.com/emirpasic/gods/sets"
	"github.com/emirpasic/gods/sets/treeset"
	"primamateria.systems/materia/pkg/manifests"
)

var ErrServiceNotFound = errors.New("service not found")

type ServiceSet struct {
	sets.Set
}

func compareServiceResource(left, right interface{}) int {
	leftSRC := left.(manifests.ServiceResourceConfig)
	rightSRC := right.(manifests.ServiceResourceConfig)
	return cmp.Compare(leftSRC.Service, rightSRC.Service)
}

func NewServiceSet() *ServiceSet {
	return &ServiceSet{
		treeset.NewWith(compareServiceResource),
	}
}

func (s *ServiceSet) List() []manifests.ServiceResourceConfig {
	result := make([]manifests.ServiceResourceConfig, s.Size())
	for k, v := range s.Values() {
		result[k] = v.(manifests.ServiceResourceConfig)
	}
	return result
}

func (s *ServiceSet) ListServiceNames() []string {
	result := make([]string, s.Size())
	for k, v := range s.Values() {
		result[k] = v.(manifests.ServiceResourceConfig).Service
	}
	return result
}

func (s *ServiceSet) Get(name string) (manifests.ServiceResourceConfig, error) {
	for _, v := range s.Values() {
		src := v.(manifests.ServiceResourceConfig)
		if src.Service == name {
			return src, nil
		}
	}
	return manifests.ServiceResourceConfig{}, ErrServiceNotFound
}
