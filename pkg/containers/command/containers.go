package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"primamateria.systems/materia/pkg/containers"
)

type ContainerDetails struct {
	ID      string   `json:"Id"`
	Created string   `json:"Created"`
	Path    string   `json:"Path"`
	Args    []string `json:"Args"`
	State   struct {
		Status     string    `json:"Status"`
		Running    bool      `json:"Running"`
		Paused     bool      `json:"Paused"`
		Restarting bool      `json:"Restarting"`
		OOMKilled  bool      `json:"OOMKilled"`
		Dead       bool      `json:"Dead"`
		Pid        int       `json:"Pid"`
		ConmonPid  int       `json:"ConmonPid"`
		ExitCode   int       `json:"ExitCode"`
		Error      string    `json:"Error"`
		StartedAt  string    `json:"StartedAt"`
		FinishedAt time.Time `json:"FinishedAt"`
	} `json:"State"`
	Config struct {
		Hostname   string `json:"Hostname"`
		Domainname string `json:"Domainname"`
		User       string `json:"User"`
	} `json:"Config"`
	Image                   string                      `json:"Image"`
	Pod                     string                      `json:"Pod"`
	Name                    string                      `json:"Name"`
	Driver                  string                      `json:"Driver"`
	Mounts                  []containers.ContainerMount `json:"Mounts"`
	IsInfra                 bool                        `json:"IsInfra"`
	IsService               bool                        `json:"IsService"`
	KubeExitCodePropagation string                      `json:"KubeExitCodePropagation"`
}

func loadContainer(ctx context.Context, remote bool, name string) (*containers.Container, error) {
	var result containers.Container
	result.BindMounts = make(map[string]containers.ContainerMount)
	result.Volumes = make(map[string]containers.Volume)
	inspectCmd := genCmd(ctx, remote, "inspect", "--format", "json", name)
	output, err := runCmd(inspectCmd)
	if err != nil {
		return nil, fmt.Errorf("can't inspect podman container: %w", err)
	}

	var inspectOutput []ContainerDetails
	if err := json.Unmarshal(output.Bytes(), &inspectOutput); err != nil {
		return nil, fmt.Errorf("can't decode podman container details: %w", err)
	}
	if len(inspectOutput) != 1 {
		return nil, fmt.Errorf("unusual amount of container details: %v", len(inspectOutput))
	}
	for _, m := range inspectOutput[0].Mounts {
		switch m.Type {
		case "bind":
			result.BindMounts[m.Destination] = m
		case "volume":
			result.Volumes[m.Name] = containers.Volume{
				Name:       m.Name,
				Mountpoint: m.Source,
				Driver:     m.Driver,
			}
		default:
			continue
		}
	}
	result.Name = name
	result.Hostname = inspectOutput[0].Config.Hostname
	return &result, nil
}

func (p *CommandManager) GetContainer(ctx context.Context, name string) (*containers.Container, error) {
	return loadContainer(ctx, p.remote, name)
}

func filterToArgs(c containers.ContainerListFilter) []string {
	result := []string{"ps", "--format", "json"}
	if c.All {
		result = append(result, "-a")
	}
	if c.Image != "" {
		result = append(result, fmt.Sprintf("--filter=ancestor=%v", c.Image))
	}

	if c.Volume != "" {
		result = append(result, fmt.Sprintf("--filter=volume=%v", c.Volume))
	}

	if c.Network != "" {
		result = append(result, fmt.Sprintf("--filter=network=%v", c.Network))
	}

	if c.Pod != "" {
		if c.Network != "" {
			result = append(result, fmt.Sprintf("--filter=pod=%v", c.Pod))
		}
	}

	return result
}

func (p *CommandManager) ListContainers(ctx context.Context, filter containers.ContainerListFilter) ([]*containers.Container, error) {
	args := filterToArgs(filter)
	cmd := genCmd(ctx, p.remote, args...)
	output, err := runCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf("can't list containers: %w", err)
	}
	var containerList []*containers.Container
	if err := json.Unmarshal(output.Bytes(), &containerList); err != nil {
		return nil, err
	}
	var results []*containers.Container
	for _, c := range containerList {

		loaded, err := loadContainer(ctx, p.remote, c.Id)
		if err != nil {
			return results, err
		}
		results = append(results, loaded)
	}
	return results, nil
}

func (p *CommandManager) PauseContainer(ctx context.Context, name string) error {
	cmd := genCmd(ctx, p.remote, "pause", name)
	_, err := runCmd(cmd)
	if err != nil {
		return fmt.Errorf("error pausing container: %w", err)
	}
	return nil
}

func (p *CommandManager) UnpauseContainer(ctx context.Context, name string) error {
	cmd := genCmd(ctx, p.remote, "unpause", name)
	_, err := runCmd(cmd)
	if err != nil {
		return fmt.Errorf("error unpausing container: %w", err)
	}
	return nil
}

func (p *CommandManager) ExecContainer(ctx context.Context, name string, command ...string) error {
	combined := append([]string{"exec", name}, command...)
	cmd := genCmd(ctx, p.remote, combined...)
	_, err := runCmd(cmd)
	if err != nil {
		return fmt.Errorf("error running command in container: %w", err)
	}
	return nil
}
