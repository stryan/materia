package main

type Resource struct {
	Path     string
	Name     string
	Kind     ResourceType
	Template bool
}

type ResourceType int

const (
	ResourceTypeContainer ResourceType = iota
	ResourceTypeVolume
	ResourceTypePod
	ResourceTypeNetwork
	ResourceTypeKube
	ResourceTypeFile

	// special types that exist after systemctl daemon-reload
	ResourceTypeService
)
