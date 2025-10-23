package containers

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

	"github.com/charmbracelet/log"
)

var supportedVolumeDumpExts = []string{"tar", "tar.gz", "tgz", "bzip", "tar.xz", "txz"}

type PodmanManager struct {
	secretsPrefix string
	remote        bool
}

type Container struct {
	Name    string
	State   string // TODO
	Volumes map[string]Volume
}
type Volume struct {
	Name       string `json:"Name"`
	Mountpoint string `json:"Mountpoint"`
	Driver     string `json:"Driver"`
}

type NetworkContainer struct {
	Name string
}
type Network struct {
	Name       string
	Containers map[string]NetworkContainer
}

type PodmanSecret struct {
	Name  string
	Value string
}

type SecretInfo struct {
	ID        string `json:"ID"`
	CreatedAt string `json:"CreatedAt"`
	UpdatedAt string `json:"UpdatedAt"`
	Spec      struct {
		SpecName string `json:"Name"`
		Driver   struct {
			DriverName string `json:"Name"`
			Options    struct {
				Path string `json:"path"`
			} `json:"Options"`
		} `json:"Driver"`
		Labels struct{} `json:"Labels"`
	} `json:"Spec"`
	SecretData string `json:"SecretData"`
}

func NewPodmanManager(remote bool, prefix string) (*PodmanManager, error) {
	p := &PodmanManager{
		secretsPrefix: prefix,
	}
	return p, nil
}

func (p *PodmanManager) PauseContainer(ctx context.Context, name string) error {
	cmd := p.genCmd(ctx, "pause", name)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error pausing container: %w", err)
	}
	if err = parsePodmanError(output); err != nil {
		return err
	}
	return nil
}

func (p *PodmanManager) UnpauseContainer(ctx context.Context, name string) error {
	cmd := p.genCmd(ctx, "unpause", name)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error unpausing container: %w", err)
	}
	if err = parsePodmanError(output); err != nil {
		return err
	}
	return nil
}

func (p *PodmanManager) InspectVolume(ctx context.Context, name string) (*Volume, error) {
	cmd := p.genCmd(ctx, "volume", "inspect", "--format", "json", name)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error inspecting volume %w", err)
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

func (p *PodmanManager) ListVolumes(ctx context.Context) ([]*Volume, error) {
	cmd := p.genCmd(ctx, "volume", "ls", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error listing volumes: %v", err)
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

func (p *PodmanManager) DumpVolume(ctx context.Context, volume *Volume, outputDir string, compressed bool) error {
	exportCmd := p.genCmd(ctx, "volume", "export", volume.Name)
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
	cmd := p.genCmd(ctx, "volume", "mount", volume.Name)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error mounting volume: %w", err)
	}
	if err = parsePodmanError(output); err != nil {
		return err
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
	cmd := p.genCmd(ctx, "volume", "import", volume.Name, sourcePath)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error importing volume: %v", err)
	}
	if err = parsePodmanError(output); err != nil {
		return err
	}

	return nil
}

func (p *PodmanManager) RemoveVolume(ctx context.Context, volume *Volume) error {
	cmd := p.genCmd(ctx, "volume", "rm", volume.Name)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error removing volume: %w", err)
	}
	if err = parsePodmanError(output); err != nil {
		return err
	}

	return nil
}

func (p *PodmanManager) ListSecrets(ctx context.Context) ([]string, error) {
	cmd := p.genCmd(ctx, "secret", "ls", "--noheading", "--format", "\"{{ range . }}{{.Name}}\\n{{end -}}\"", "--filter", fmt.Sprintf("name=%v*", p.secretsPrefix))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error listing secrets: %w", err)
	}
	if err = parsePodmanError(output); err != nil {
		return nil, err
	}
	var result []string
	// TODO clean this up
	for v := range strings.SplitSeq(string(output), "\n") {
		v = strings.Trim(v, " \t\n\r\"'")
		if v != "" {
			result = append(result, strings.TrimPrefix(v, p.secretsPrefix))
		}
	}
	return result, nil
}

func (p *PodmanManager) GetSecret(ctx context.Context, secretName string) (*PodmanSecret, error) {
	cmd := p.genCmd(ctx, "secret", "inspect", "--showsecret", fmt.Sprintf("%v%v", p.secretsPrefix, secretName))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error getting podman secret: %w", err)
	}
	if err = parsePodmanError(output); err != nil {
		return nil, err
	}
	var infos []*SecretInfo
	if err := json.Unmarshal(output, &infos); err != nil {
		return nil, err
	}
	return &PodmanSecret{Name: secretName, Value: infos[0].SecretData}, nil
}

func (p *PodmanManager) WriteSecret(ctx context.Context, secretName, secretValue string) error {
	cmd := p.genCmd(ctx, "secret", "create", "--replace", fmt.Sprintf("%v%v", p.secretsPrefix, secretName), "-")
	var valBuf bytes.Buffer
	valBuf.Write([]byte(secretValue))
	cmd.Stdin = &valBuf
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error writing podman secret: %w", err)
	}
	return parsePodmanError(output)
}

func (p *PodmanManager) RemoveSecret(ctx context.Context, secretName string) error {
	cmd := p.genCmd(ctx, "secret", "rm", fmt.Sprintf("%v%v", p.secretsPrefix, secretName))
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error removing podman secret: %w", err)
	}
	return parsePodmanError(output)
}

func (p *PodmanManager) ListNetworks(ctx context.Context) ([]*Network, error) {
	cmd := p.genCmd(ctx, "network", "ls", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error listing podman networks: %w", err)
	}
	if err = parsePodmanError(output); err != nil {
		return nil, err
	}
	var networks []*Network
	if err := json.Unmarshal(output, &networks); err != nil {
		return nil, err
	}
	return networks, nil
}

func (p *PodmanManager) RemoveNetwork(ctx context.Context, n *Network) error {
	cmd := p.genCmd(ctx, "network", "rm", n.Name)
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	if err = parsePodmanError(output); err != nil {
		return err
	}
	return nil
}

func (p *PodmanManager) Close() {
}

func (p *PodmanManager) SecretName(name string) string {
	return fmt.Sprintf("%v%v", p.secretsPrefix, name)
}

func parsePodmanError(rawerror []byte) error {
	errorString := string(rawerror)
	if realErr, found := strings.CutPrefix(errorString, "Error: "); found {
		return fmt.Errorf("error from podman command: %v", realErr)
	}
	return nil
}

func (p *PodmanManager) genCmd(ctx context.Context, args ...string) *exec.Cmd {
	if p.remote {
		argsWithRemote := append([]string{"--remote"}, args...)
		return exec.CommandContext(ctx, "podman", argsWithRemote...)
	}
	return exec.CommandContext(ctx, "podman", args...)
}
