package materia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
)

type Containers interface {
	InspectVolume(string) (*Volume, error)
	InstallFile(context.Context, *Component, Resource) error
	RemoveFile(context.Context, *Component, Resource) error
	ListVolumes(context.Context) ([]*Volume, error)
	Close()
}

type PodmanManager struct{}

type Volume struct {
	Name       string `json:"Name"`
	Mountpoint string `json:"Mountpoint"`
}

func NewPodmanManager(cfg *Config) (*PodmanManager, error) {
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

func (p *PodmanManager) InstallFile(ctx context.Context, parent *Component, res Resource) error {
	var vrConf *VolumeResourceConfig
	for _, vr := range parent.VolumeResources {
		if vr.Resource == res.Name {
			vrConf = &vr
			break
		}
	}
	if vrConf == nil {
		return fmt.Errorf("tried to install volume file for nonexistent volume resource: %v", res.Name)
	}
	vrConf.Volume = fmt.Sprintf("systemd-%v", vrConf.Volume)
	volumes, err := p.ListVolumes(ctx)
	if err != nil {
		return err
	}
	var volume *Volume
	if !slices.ContainsFunc(volumes, func(v *Volume) bool {
		if v.Name == vrConf.Volume {
			volume = v
			return true
		}
		return false
	}) {
		return fmt.Errorf("tried to install volume file into nonexistent volume: %v/%v", vrConf.Volume, res.Name)
	}
	inVolumeLoc := filepath.Join(volume.Mountpoint, vrConf.Path)
	data, err := os.ReadFile(res.Path)
	if err != nil {
		return err
	}
	mode := vrConf.Mode
	if mode == "" {
		mode = "0o755"
	}
	parsedMode, err := strconv.ParseInt(mode, 8, 32)
	if err != nil {
		return err
	}
	err = os.WriteFile(inVolumeLoc, bytes.NewBuffer(data).Bytes(), os.FileMode(parsedMode))
	if err != nil {
		return err
	}
	if vrConf.Owner != "" {
		uid, err := strconv.ParseInt(vrConf.Owner, 10, 32)
		if err != nil {
			return err
		}
		err = os.Chown(inVolumeLoc, int(uid), -1)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *PodmanManager) RemoveFile(ctx context.Context, parent *Component, res Resource) error {
	var vrConf *VolumeResourceConfig
	for _, vr := range parent.VolumeResources {
		if vr.Resource == res.Name {
			vrConf = &vr
		}
	}
	if vrConf == nil {
		return fmt.Errorf("tried to remove volume file for nonexistent volume resource: /%v", res.Name)
	}
	vrConf.Volume = fmt.Sprintf("systemd-%v", vrConf.Volume)
	volumes, err := p.ListVolumes(ctx)
	if err != nil {
		return err
	}
	var volume *Volume
	if !slices.ContainsFunc(volumes, func(v *Volume) bool {
		if v.Name == vrConf.Volume {
			volume = v
			return true
		}
		return false
	}) {
		return fmt.Errorf("tried to remove volume file into nonexistent volume: %v/%v", vrConf.Volume, res.Name)
	}
	inVolumeLoc := filepath.Join(volume.Mountpoint, vrConf.Path)
	return os.Remove(inVolumeLoc)
}

func (p *PodmanManager) Close() {
}
