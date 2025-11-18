package components

import (
	"errors"
)

type ResourceSet struct {
	set   map[string]Resource
	order []string
	size  int
}

func NewResourceSet() *ResourceSet {
	return &ResourceSet{
		set:  make(map[string]Resource),
		size: 0,
	}
}

func (r *ResourceSet) Add(res Resource) error {
	if _, ok := r.set[res.Path]; ok {
		return errors.New("resource already in set")
	}
	r.set[res.Path] = res
	r.size++
	r.order = append(r.order, res.Path)
	return nil
}

func (r *ResourceSet) Delete(name string) {
	if _, ok := r.set[name]; !ok {
		return
	}
	r.size--
	for k, v := range r.order {
		if v == name {
			r.order[k] = ""
		}
	}
	delete(r.set, name)
}

func (r *ResourceSet) Size() int {
	return r.size
}

func (r *ResourceSet) List() []Resource {
	result := make([]Resource, r.size)
	for i, k := range r.order {
		if k != "" {
			result[i] = r.set[k]
		}
	}
	return result
}

func (r *ResourceSet) Get(name string) (Resource, error) {
	if res, ok := r.set[name]; !ok {
		return Resource{}, errors.New("resource not found")
	} else {
		return res, nil
	}
}

func (r *ResourceSet) Set(res Resource) {
	if _, ok := r.set[res.Path]; !ok {
		r.order = append(r.order, res.Path)
		r.size++
	}
	r.set[res.Path] = res
}
