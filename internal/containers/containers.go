package containers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ContainerManager interface {
	InspectVolume(string) (*Volume, error)
	ListVolumes(context.Context) ([]*Volume, error)
	PauseContainer(context.Context, string) error
	UnpauseContainer(context.Context, string) error
	DumpVolume(context.Context, Volume, string, bool) error
	Close()
}

type PodmanManager struct{}

type Container struct {
	Name    string
	State   string // TODO
	Volumes map[string]Volume
}
type Volume struct {
	Name       string `json:"Name"`
	Mountpoint string `json:"Mountpoint"`
}

func NewPodmanManager() (*PodmanManager, error) {
	return &PodmanManager{}, nil
}

func (p *PodmanManager) PauseContainer(_ context.Context, name string) error {
	cmd := exec.Command("podman", "pause", name)
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	if err = parsePodmanError(output); err != nil {
		return err
	}
	return nil
}

func (p *PodmanManager) UnpauseContainer(_ context.Context, name string) error {
	cmd := exec.Command("podman", "unpause", name)
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	if err = parsePodmanError(output); err != nil {
		return err
	}
	return nil
}

func (p *PodmanManager) InspectVolume(name string) (*Volume, error) {
	cmd := exec.Command("podman", "volume", "inspect", "--format", "json", name)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if err = parsePodmanError(output); err != nil {
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
	if err = parsePodmanError(output); err != nil {
		return nil, err
	}
	var volumes []*Volume
	if err := json.Unmarshal(output, &volumes); err != nil {
		return nil, err
	}
	return volumes, nil
}

func (p *PodmanManager) DumpVolume(_ context.Context, volume Volume, outputDir string, compressed bool) error {
	exportCmd := exec.Command("podman", "volume", "export", volume.Name)
	compressCmd := exec.Command("zstd")
	outputFilename := filepath.Join(outputDir, volume.Name)
	outputFilename = fmt.Sprintf("%v.tar", outputFilename)
	if compressed {
		outputFilename = fmt.Sprintf("%v.zstd", outputFilename)
	}
	outfile, err := os.Create(outputFilename)
	if err != nil {
		return err
	}
	defer func() { _ = outfile.Close() }()
	if compressed {
		compressCmd.Stdin, err = exportCmd.StdoutPipe()
		if err != nil {
			return err
		}
		compressCmd.Stdout = outfile
		err = compressCmd.Start()
		if err != nil {
			return err
		}
		err = exportCmd.Run()
		if err != nil {
			return err
		}
		err = compressCmd.Wait()
		if err != nil {
			return err
		}
		return nil
	}
	exportCmd.Stdout = outfile
	err = exportCmd.Start()
	if err != nil {
		return err
	}
	err = exportCmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

func (p *PodmanManager) Close() {
}

func parsePodmanError(rawerror []byte) error {
	errorString := string(rawerror)
	if strings.HasPrefix(errorString, "Error: ") {
		return errors.New(strings.TrimPrefix(errorString, "Error: "))
	}
	return nil
}
