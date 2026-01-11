package containers

import (
	"context"
	"encoding/json"
	"fmt"
)

type Container struct {
	Name    string
	State   string // TODO
	Volumes map[string]Volume
}

type ContainerListFilter struct {
	Image   string
	Volume  string
	Network string
	Pod     string
	All     bool
}

func (c ContainerListFilter) ToArgs() []string {
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

func (p *PodmanManager) ListContainers(ctx context.Context, filter ContainerListFilter) ([]*Container, error) {
	args := filter.ToArgs()
	cmd := genCmd(ctx, p.remote, args...)
	output, err := runCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf("can't list containers: %w", err)
	}
	var containers []*Container
	if err := json.Unmarshal(output.Bytes(), &containers); err != nil {
		return nil, err
	}
	return containers, nil
}

func (p *PodmanManager) PauseContainer(ctx context.Context, name string) error {
	cmd := genCmd(ctx, p.remote, "pause", name)
	_, err := runCmd(cmd)
	if err != nil {
		return fmt.Errorf("error pausing container: %w", err)
	}
	return nil
}

func (p *PodmanManager) UnpauseContainer(ctx context.Context, name string) error {
	cmd := genCmd(ctx, p.remote, "unpause", name)
	_, err := runCmd(cmd)
	if err != nil {
		return fmt.Errorf("error unpausing container: %w", err)
	}
	return nil
}
