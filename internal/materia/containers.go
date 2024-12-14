package materia

import (
	"context"
	"fmt"

	"github.com/containers/podman/v4/pkg/bindings/volumes"
)

type Containers interface {
	Inspect(string) (*Volume, error)
}

type PodmanManager struct {
	Conn context.Context
}

type Volume struct {
	Name, Mountpoint string
}

func (p *PodmanManager) Inspect(name string) (*Volume, error) {
	vol, err := volumes.Inspect(p.Conn, fmt.Sprintf("systemd-%v", name), nil)
	if err != nil {
		return nil, err
	}
	return &Volume{
		Name:       vol.Name,
		Mountpoint: vol.Mountpoint,
	}, nil
}
