package containers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/charmbracelet/log"
)

var supportedVolumeDumpExts = []string{"tar", "tar.gz", "tgz", "bzip", "tar.xz", "txz"}

type Volume struct {
	Name       string `json:"Name"`
	Mountpoint string `json:"Mountpoint"`
	Driver     string `json:"Driver"`
}

func (p *PodmanManager) GetVolume(ctx context.Context, name string) (*Volume, error) {
	cmd := genCmd(ctx, p.remote, "volume", "inspect", "--format", "json", name)
	output, err := runCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf("error inspecting volume %w", err)
	}

	var volume []Volume

	if err := json.Unmarshal(output.Bytes(), &volume); err != nil {
		return nil, err
	}
	if len(volume) != 1 {
		return nil, ErrPodmanObjectNotFound
	}

	return &volume[0], nil
}

func (p *PodmanManager) ListVolumes(ctx context.Context) ([]*Volume, error) {
	cmd := genCmd(ctx, p.remote, "volume", "ls", "--format", "json")
	output, err := runCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf("error listing volumes: %v", err)
	}
	var volumes []*Volume
	if err := json.Unmarshal(output.Bytes(), &volumes); err != nil {
		return nil, err
	}
	return volumes, nil
}

func (p *PodmanManager) DumpVolume(ctx context.Context, volume *Volume, outputDir string, compressed bool) error {
	exportCmd := genCmd(ctx, p.remote, "volume", "export", volume.Name)
	compressCmd := exec.CommandContext(ctx, "zstd")
	outputFilename := filepath.Join(outputDir, volume.Name)
	outputFilename = fmt.Sprintf("%v.tar", outputFilename)
	if compressed {
		outputFilename = fmt.Sprintf("%v.zstd", outputFilename)
	}
	log.Debugf("dumping volume %v to path %v", volume.Name, outputFilename)
	outfile, err := os.Create(outputFilename)
	if err != nil {
		return fmt.Errorf("error creating output file name: %w", err)
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
			return fmt.Errorf("error starting volume compression command: %w", err)
		}
		err = exportCmd.Run()
		if err != nil {
			return fmt.Errorf("error running volume export: %w", err)
		}
		err = compressCmd.Wait()
		if err != nil {
			return fmt.Errorf("error with volume compression: %w", err)
		}
		return nil
	}
	exportCmd.Stdout = outfile
	err = exportCmd.Run()
	if err != nil {
		return fmt.Errorf("error starting export command: %w", err)
	}
	return nil
}

func (p *PodmanManager) MountVolume(ctx context.Context, volume *Volume) error {
	cmd := genCmd(ctx, p.remote, "volume", "mount", volume.Name)
	_, err := runCmd(cmd)
	if err != nil {
		return fmt.Errorf("error mounting volume: %w", err)
	}

	return nil
}

func (p *PodmanManager) ImportVolume(ctx context.Context, volume *Volume, sourcePath string) error {
	if slices.Contains(supportedVolumeDumpExts, filepath.Ext(sourcePath)) {
		return fmt.Errorf("unsupported volume dump type %v for import", filepath.Ext(sourcePath))
	}
	if volume.Driver != "local" && volume.Driver != "" {
		return errors.New("can only import into local volume")
	}
	cmd := genCmd(ctx, p.remote, "volume", "import", volume.Name, sourcePath)
	_, err := runCmd(cmd)
	if err != nil {
		return fmt.Errorf("error importing volume: %v", err)
	}

	return nil
}

func (p *PodmanManager) RemoveVolume(ctx context.Context, volume *Volume) error {
	cmd := genCmd(ctx, p.remote, "volume", "rm", volume.Name)
	_, err := runCmd(cmd)
	if err != nil {
		return fmt.Errorf("error removing volume: %w", err)
	}

	return nil
}
