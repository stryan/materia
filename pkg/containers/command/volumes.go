package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/log/v2"
	"primamateria.systems/materia/pkg/containers"
)

var supportedVolumeDumpExts = []string{".tar", ".tar.gz", ".zst", ".zstd", ".gz"}

func (p *CommandManager) GetVolume(ctx context.Context, name string) (*containers.Volume, error) {
	cmd := genCmd(ctx, p.remote, "volume", "inspect", "--format", "json", name)
	output, err := runCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf("error inspecting volume %w", err)
	}

	var volume []containers.Volume

	if err := json.Unmarshal(output.Bytes(), &volume); err != nil {
		return nil, err
	}
	if len(volume) != 1 {
		return nil, containers.ErrPodmanObjectNotFound
	}

	return &volume[0], nil
}

func (p *CommandManager) ListVolumes(ctx context.Context) ([]*containers.Volume, error) {
	cmd := genCmd(ctx, p.remote, "volume", "ls", "--format", "json")
	output, err := runCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf("error listing volumes: %v", err)
	}
	var volumes []*containers.Volume
	if err := json.Unmarshal(output.Bytes(), &volumes); err != nil {
		return nil, err
	}
	return volumes, nil
}

func (p *CommandManager) DumpVolume(ctx context.Context, volume *containers.Volume, outputDir string) error {
	exportCmd := genCmd(ctx, p.remote, "volume", "export", volume.Name)
	outputFilename := filepath.Join(outputDir, volume.Name)
	outputFilename = fmt.Sprintf("%v-volume.tar", outputFilename)
	if p.compression != "" {
		suffix := "gz"
		if p.compression == "zstd" {
			suffix = "zst"
		}
		outputFilename = fmt.Sprintf("%v.%v", outputFilename, suffix)
	}
	log.Debugf("dumping volume %v to path %v", volume.Name, outputFilename)
	outfile, err := os.Create(outputFilename)
	if err != nil {
		return fmt.Errorf("error creating output file name: %w", err)
	}
	defer func() { _ = outfile.Close() }()
	if p.compression != "" {
		cmd := "gz"
		if p.compression == "zstd" {
			cmd = "zstd"
		}
		compressCmd := exec.CommandContext(ctx, cmd)
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
	errorout := bytes.NewBuffer([]byte{})
	exportCmd.Stderr = errorout

	err = exportCmd.Run()
	if err != nil {
		errString := errorout.String()
		if realErr, found := strings.CutPrefix(errString, "Error: "); found {
			return fmt.Errorf("err %w: error from podman command: %v", err, realErr)
		} else {
			return fmt.Errorf("error starting export command: %w", err)
		}
	}
	return nil
}

func (p *CommandManager) MountVolume(ctx context.Context, volume *containers.Volume) error {
	cmd := genCmd(ctx, p.remote, "volume", "mount", volume.Name)
	_, err := runCmd(cmd)
	if err != nil {
		return fmt.Errorf("error mounting volume: %w", err)
	}

	return nil
}

func (p *CommandManager) ImportVolume(ctx context.Context, volume *containers.Volume, sourcePath string) error {
	ext := filepath.Ext(sourcePath)
	if !slices.Contains(supportedVolumeDumpExts, ext) {
		return fmt.Errorf("unsupported volume dump type %v for import. Supported types: %v", filepath.Ext(sourcePath), supportedVolumeDumpExts)
	}
	if volume.Driver != "local" && volume.Driver != "" {
		return errors.New("can only import into local volume")
	}
	if ext == ".tar" {
		cmd := genCmd(ctx, p.remote, "volume", "import", volume.Name, sourcePath)
		_, err := runCmd(cmd)
		if err != nil {
			return fmt.Errorf("error importing volume: %v", err)
		}
		return nil
	}
	var dcmd string
	switch ext {
	case ".zstd", ".zst":
		dcmd = "zstd"
	case ".gz", "tar.gz":
		dcmd = "gzip"
	default:
		return fmt.Errorf("unsupported volume backup extension: %v", ext)
	}

	decompressCmd := exec.CommandContext(ctx, dcmd, "-c", "-d", sourcePath)
	decerrorout := bytes.NewBuffer([]byte{})
	decompressCmd.Stderr = decerrorout
	// TODO need to catch decompressCmd errors
	importCommand := genCmd(ctx, p.remote, "volume", "import", volume.Name, "-")
	var err error
	importCommand.Stdin, err = decompressCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to pipe import decompression command: %w", err)
	}
	errorout := bytes.NewBuffer([]byte{})
	importCommand.Stderr = errorout
	err = importCommand.Start()
	if err != nil {
		return fmt.Errorf("unable to start import: %w", err)
	}
	err = decompressCmd.Run()
	if err != nil {
		return fmt.Errorf("unable to start volume decompression: %w", err)
	}
	err = importCommand.Wait()
	if err != nil {
		errString := errorout.String()
		if realErr, found := strings.CutPrefix(errString, "Error: "); found {
			return fmt.Errorf("err %w: error from podman command: %v", err, realErr)
		} else {
			return fmt.Errorf("error starting import command: %w", err)
		}
	}
	return nil
}

func (p *CommandManager) RemoveVolume(ctx context.Context, volume *containers.Volume) error {
	cmd := genCmd(ctx, p.remote, "volume", "rm", volume.Name)
	_, err := runCmd(cmd)
	if err != nil {
		return fmt.Errorf("error removing volume: %w", err)
	}

	return nil
}
