package containers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type PodmanManager struct {
	secretsPrefix string
}

type Container struct {
	Name    string
	State   string // TODO
	Volumes map[string]Volume
}
type Volume struct {
	Name       string `json:"Name"`
	Mountpoint string `json:"Mountpoint"`
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

func NewPodmanManager() (*PodmanManager, error) {
	return &PodmanManager{secretsPrefix: "materia-"}, nil
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

func (p *PodmanManager) ListSecrets(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "podman", "secret", "ls", "--noheading", "--format", "\"{{ range . }}{{.Name}}\\n{{end -}}\"", "--filter", fmt.Sprintf("name=%v*", p.secretsPrefix))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if err = parsePodmanError(output); err != nil {
		return nil, err
	}
	var result []string
	// TODO clean this up
	for v := range strings.SplitSeq(string(output), "\n") {
		v := strings.Trim(v, " \t\n\r\"'")
		if v != "" {
			result = append(result, strings.TrimSpace(v))
		}
	}
	return result, nil
}

func (p *PodmanManager) GetSecret(ctx context.Context, secretName string) (*PodmanSecret, error) {
	cmd := exec.CommandContext(ctx, "podman", "secret", "inspect", "--showsecret", fmt.Sprintf("%v%v", p.secretsPrefix, secretName))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
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
	cmd := exec.CommandContext(ctx, "podman", "secret", "create", "--replace", fmt.Sprintf("%v%v", p.secretsPrefix, secretName), "-")
	var valBuf bytes.Buffer
	valBuf.Write([]byte(secretValue))
	cmd.Stdin = &valBuf
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	return parsePodmanError(output)
}

func (p *PodmanManager) Close() {
}

func parsePodmanError(rawerror []byte) error {
	errorString := string(rawerror)
	if realErr, found := strings.CutPrefix(errorString, "Error: "); found {
		return fmt.Errorf("error from podman command: %v", realErr)
	}
	return nil
}
