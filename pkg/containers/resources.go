package containers

import "errors"

type Container struct {
	Name       string
	Id         string
	Hostname   string
	Volumes    map[string]Volume
	BindMounts map[string]ContainerMount
}

type ContainerMount struct {
	Type        string   `json:"Type"`
	Name        string   `json:"Name"`
	Source      string   `json:"Source"`
	Destination string   `json:"Destination"`
	Driver      string   `json:"Driver"`
	Mode        string   `json:"Mode"`
	Options     []string `json:"Options"`
	Rw          bool     `json:"RW"`
	Propagation string   `json:"Propagation"`
}

type ContainerListFilter struct {
	Image   string
	Volume  string
	Network string
	Pod     string
	All     bool
}

type Image struct {
	Names []string `json:"Names"`
	ID    string   `json:"ID"`
}

type Network struct {
	Name       string
	Containers []NetworkContainer
}

type NetworkContainer struct {
	Name string `json:"name"`
}

type PodmanSecret struct {
	Name  string
	Value string
}

type Volume struct {
	Name       string `json:"Name"`
	Mountpoint string `json:"Mountpoint"`
	Driver     string `json:"Driver"`
}

var ErrPodmanObjectNotFound error = errors.New("no such object")
