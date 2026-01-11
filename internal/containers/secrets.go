package containers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

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

func (p *PodmanManager) ListSecrets(ctx context.Context) ([]string, error) {
	cmd := genCmd(ctx, p.remote, "secret", "ls", "--noheading", "--format", "\"{{ range . }}{{.Name}}\\n{{end -}}\"", "--filter", fmt.Sprintf("name=%v*", p.secretsPrefix))
	output, err := runCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf("error listing secrets: %w", err)
	}
	var result []string
	// TODO clean this up
	for v := range strings.SplitSeq(output.String(), "\n") {
		v = strings.Trim(v, " \t\n\r\"'")
		if v != "" {
			result = append(result, strings.TrimPrefix(v, p.secretsPrefix))
		}
	}
	return result, nil
}

func (p *PodmanManager) GetSecret(ctx context.Context, secretName string) (*PodmanSecret, error) {
	cmd := genCmd(ctx, p.remote, "secret", "inspect", "--showsecret", fmt.Sprintf("%v%v", p.secretsPrefix, secretName))
	output, err := runCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf("error getting podman secret: %w", err)
	}
	var infos []*SecretInfo
	if err := json.Unmarshal(output.Bytes(), &infos); err != nil {
		return nil, err
	}
	return &PodmanSecret{Name: secretName, Value: infos[0].SecretData}, nil
}

func (p *PodmanManager) WriteSecret(ctx context.Context, secretName, secretValue string) error {
	cmd := genCmd(ctx, p.remote, "secret", "create", "--replace", fmt.Sprintf("%v%v", p.secretsPrefix, secretName), "-")
	var valBuf bytes.Buffer
	valBuf.Write([]byte(secretValue))
	cmd.Stdin = &valBuf
	_, err := runCmd(cmd)
	if err != nil {
		return fmt.Errorf("error writing podman secret: %w", err)
	}
	return nil
}

func (p *PodmanManager) RemoveSecret(ctx context.Context, secretName string) error {
	cmd := genCmd(ctx, p.remote, "secret", "rm", fmt.Sprintf("%v%v", p.secretsPrefix, secretName))
	_, err := runCmd(cmd)
	if err != nil {
		return fmt.Errorf("error removing podman secret: %w", err)
	}
	return nil
}

func (p *PodmanManager) SecretName(name string) string {
	return fmt.Sprintf("%v%v", p.secretsPrefix, name)
}
