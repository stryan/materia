package services

import (
	"cmp"

	"github.com/emirpasic/gods/sets"
	"github.com/emirpasic/gods/sets/treeset"
)

type ServiceSet struct {
	sets.Set
}

func compareServiceResource(left, right interface{}) int {
	leftSRC := left.(Service)
	rightSRC := right.(Service)
	return cmp.Compare(leftSRC.Name, rightSRC.Name)
}

func NewServiceSet() *ServiceSet {
	return &ServiceSet{
		treeset.NewWith(compareServiceResource),
	}
}

func (s *ServiceSet) Add(serv Service) {
	s.Set.Add(serv)
}

func (s *ServiceSet) List() []Service {
	result := make([]Service, s.Size())
	for k, v := range s.Values() {
		result[k] = v.(Service)
	}
	return result
}

func (s *ServiceSet) ListServiceNames() []string {
	result := make([]string, s.Size())
	for k, v := range s.Values() {
		result[k] = v.(Service).Name
	}
	return result
}

func (s *ServiceSet) Get(name string) (Service, error) {
	for _, v := range s.Values() {
		src := v.(Service)
		if src.Name == name {
			return src, nil
		}
	}
	return Service{}, ErrServiceNotFound
}

func (r *ServiceSet) Union(other *ServiceSet) *ServiceSet {
	resultSet := NewServiceSet()

	for _, element := range r.List() {
		resultSet.Add(element)
	}
	for _, element := range other.List() {
		resultSet.Add(element)
	}

	return resultSet
}

func (r *ServiceSet) Intersection(other *ServiceSet) *ServiceSet {
	resultSet := NewServiceSet()

	for _, element := range r.List() {
		if other.Contains(element) {
			resultSet.Add(element)
		}
	}

	return resultSet
}

func (r *ServiceSet) Difference(other *ServiceSet) *ServiceSet {
	resultSet := NewServiceSet()

	for _, element := range r.List() {
		if !other.Contains(element) {
			resultSet.Add(element)
		}
	}
	return resultSet
}
