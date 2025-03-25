package containers

import (
	"context"
	"encoding/json"
	"os/exec"
)

type Containers interface {
	InspectVolume(string) (*Volume, error)
	ListVolumes(context.Context) ([]*Volume, error)
	Close()
}

type PodmanManager struct{}

type Volume struct {
	Name       string `json:"Name"`
	Mountpoint string `json:"Mountpoint"`
}

func NewPodmanManager() (*PodmanManager, error) {
	return &PodmanManager{}, nil
}

func (p *PodmanManager) InspectVolume(name string) (*Volume, error) {
	cmd := exec.Command("podman", "volume", "inspect", "--format", "json", name)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var volume []Volume

	if err := json.Unmarshal(output, &volume); err != nil {
		return nil, err
	}

	return &volume[0], nil
}

func (p *PodmanManager) ListVolumes(_ context.Context) ([]*Volume, error) {
	cmd := exec.Command("podman", "volume", "ls", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var volumes []*Volume
	if err := json.Unmarshal(output, &volumes); err != nil {
		return nil, err
	}
	return volumes, nil
}

func (p *PodmanManager) Close() {
}
