package components

import (
	"errors"

	"github.com/emirpasic/gods/maps"
	"github.com/emirpasic/gods/maps/linkedhashmap"
)

var ErrElementNotFound error = errors.New("ErrElementNotFound")

type ResourceSet struct {
	newSet maps.Map
}

func NewResourceSet() *ResourceSet {
	return &ResourceSet{
		newSet: linkedhashmap.New(),
	}
}

func (r *ResourceSet) Add(res Resource) error {
	if _, ok := r.newSet.Get(res.Path); ok {
		return errors.New("resource already in set")
	}
	r.newSet.Put(res.Path, res)
	return nil
}

func (r *ResourceSet) Delete(name string) {
	r.newSet.Remove(name)
}

func (r *ResourceSet) Size() int {
	return r.newSet.Size()
}

func (r *ResourceSet) List() []Resource {
	result := make([]Resource, r.newSet.Size())
	for i, v := range r.newSet.Values() {
		result[i] = v.(Resource)
	}
	return result
}

func (r *ResourceSet) Contains(name string) bool {
	if _, ok := r.newSet.Get(name); ok {
		return true
	}
	return false
}

func (r *ResourceSet) Get(name string) (Resource, error) {
	if res, ok := r.newSet.Get(name); !ok {
		return Resource{}, ErrElementNotFound
	} else {
		return res.(Resource), nil
	}
}

func (r *ResourceSet) Set(res Resource) {
	r.newSet.Put(res.Path, res)
}

func (r *ResourceSet) Union(other *ResourceSet) *ResourceSet {
	resultSet := NewResourceSet()

	for _, element := range r.List() {
		_ = resultSet.Add(element)
	}
	for _, element := range other.List() {
		_ = resultSet.Add(element)
	}

	return resultSet
}

func (r *ResourceSet) Intersection(other *ResourceSet) *ResourceSet {
	resultSet := NewResourceSet()

	for _, element := range r.List() {
		if other.Contains(element.Path) {
			_ = resultSet.Add(element)
		}
	}

	return resultSet
}

func (r *ResourceSet) Difference(other *ResourceSet) *ResourceSet {
	resultSet := NewResourceSet()

	for _, element := range r.List() {
		if !other.Contains(element.Path) {
			_ = resultSet.Add(element)
		}
	}
	return resultSet
}
