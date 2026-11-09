package components

import (
	"cmp"
	"errors"

	"github.com/emirpasic/gods/sets"
	"github.com/emirpasic/gods/sets/treeset"
	"primamateria.systems/materia/pkg/manifests"
)

var ErrServiceManifestNotFound = errors.New("service manifest not found")

type ServiceManifestSet struct {
	sets.Set
}

func compareServiceManifest(left, right any) int {
	leftSM := left.(manifests.ServiceManifest)
	rightSM := right.(manifests.ServiceManifest)
	return cmp.Compare(leftSM.Service, rightSM.Service)
}

func NewServiceManifestSet() *ServiceManifestSet {
	return &ServiceManifestSet{
		treeset.NewWith(compareServiceManifest),
	}
}

func (s *ServiceManifestSet) List() []manifests.ServiceManifest {
	result := make([]manifests.ServiceManifest, s.Size())
	for k, v := range s.Values() {
		result[k] = v.(manifests.ServiceManifest)
	}
	return result
}

func (s *ServiceManifestSet) ListServiceNames() []string {
	result := make([]string, s.Size())
	for k, v := range s.Values() {
		result[k] = v.(manifests.ServiceManifest).Service
	}
	return result
}

func (s *ServiceManifestSet) Get(name string) (manifests.ServiceManifest, error) {
	for _, v := range s.Values() {
		src := v.(manifests.ServiceManifest)
		if src.Service == name {
			return src, nil
		}
	}
	return manifests.ServiceManifest{}, ErrServiceManifestNotFound
}
