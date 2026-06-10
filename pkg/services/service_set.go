package services

import (
	"cmp"
	"errors"

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

func (s *ServiceSet) Add(serv Service) error {
	if s.Contains(serv) {
		return errors.New("service already in set")
	}
	s.Set.Add(serv)
	return nil
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
