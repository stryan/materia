package native

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"charm.land/log/v2"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	vm "go.podman.io/podman/v6/pkg/bindings/volumes"
	"primamateria.systems/materia/pkg/containers"
)

var supportedVolumeDumpExts = []string{".tar", ".tar.gz", ".zst", ".zstd", ".gz"}

func (n *NativeManager) GetVolume(_ context.Context, name string) (*containers.Volume, error) {
	vol, err := vm.Inspect(n.conn, name, nil)
	if err != nil {
		return nil, err
	}
	result := &containers.Volume{
		Name:       name,
		Mountpoint: vol.Mountpoint,
		Driver:     vol.Driver,
	}
	return result, nil
}

func (n *NativeManager) ListVolumes(_ context.Context) ([]*containers.Volume, error) {
	vols, err := vm.List(n.conn, nil)
	if err != nil {
		return nil, err
	}
	result := make([]*containers.Volume, 0, len(vols))
	for _, v := range vols {
		entry := &containers.Volume{
			Name:       v.Name,
			Mountpoint: v.Mountpoint,
			Driver:     v.Driver,
		}
		result = append(result, entry)

	}
	return result, nil
}

func (n *NativeManager) RemoveVolume(_ context.Context, volume *containers.Volume) error {
	return vm.Remove(n.conn, volume.Name, nil)
}

func (n *NativeManager) ImportVolume(_ context.Context, volume *containers.Volume, sourcePath string) error {
	ext := filepath.Ext(sourcePath)
	if !slices.Contains(supportedVolumeDumpExts, ext) {
		return fmt.Errorf("unsupported volume dump type %v for import. Supported types: %v", ext, supportedVolumeDumpExts)
	}
	if volume.Driver != "local" && volume.Driver != "" {
		return fmt.Errorf("cannot import into non-local driver type :%v", volume.Driver)
	}

	f, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("opening volume dump: %w", err)
	}
	defer func() { _ = f.Close() }()

	var reader io.Reader
	switch ext {
	case ".tar":
		reader = f
	case ".gz", ".tar.gz":
		gr, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("unable to create gzip reader: %w", err)
		}
		defer func() { _ = gr.Close() }()
		reader = gr
	case ".zst", ".zstd":
		zr, err := zstd.NewReader(f)
		if err != nil {
			return fmt.Errorf("unable to create zstd reader: %w", err)
		}
		defer zr.Close()
		reader = zr
	default:
		return fmt.Errorf("unsupported volume backup extension: %v", ext)
	}

	if err := vm.Import(n.conn, volume.Name, reader); err != nil {
		return fmt.Errorf("unable to import volume %v: %w", volume.Name, err)
	}
	return nil
}

func (n *NativeManager) DumpVolume(_ context.Context, volume *containers.Volume, outputPath string) error {
	outputFilename := dumpFilename(outputPath, volume.Name, n.compression)
	outfile, err := os.Create(outputFilename)
	if err != nil {
		return fmt.Errorf("error creating output file name: %w", err)
	}
	defer func() { _ = outfile.Close() }()
	var writer io.Writer
	var compressor io.Closer
	switch n.compression {
	case "gzip":
		gz := gzip.NewWriter(outfile)
		compressor = gz
		writer = gz
	case "zstd":
		zw, err := zstd.NewWriter(outfile)
		if err != nil {
			return fmt.Errorf("unable to create zstd writer: %w", err)
		}
		compressor = zw
		writer = zw
	default:
		writer = outfile
	}
	log.Debugf("dumping volume %v to path %v", volume.Name, outputFilename)
	if err := vm.Export(n.conn, volume.Name, writer); err != nil {
		return fmt.Errorf("unable to dump volume %v: %w", volume.Name, err)
	}
	if compressor != nil {
		if err := compressor.Close(); err != nil {
			return fmt.Errorf("unable to finalize volume dump %v: %w", volume.Name, err)
		}
	}
	return nil
}

func dumpFilename(outputPath, volumeName, compression string) string {
	suffix := ""
	switch compression {
	case "gzip":
		suffix = ".gz"
	case "zstd":
		suffix = ".zst"
	}
	return filepath.Join(outputPath, fmt.Sprintf("%s-volume.tar%s", volumeName, suffix))
}
