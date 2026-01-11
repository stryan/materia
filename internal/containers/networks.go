package containers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

type Network struct {
	Name       string
	Containers []NetworkContainer
}

type NetworkContainer struct {
	Name string `json:"name"`
}

type NetworkDetails struct {
	Name             string                      `json:"name"`
	ID               string                      `json:"id"`
	Driver           string                      `json:"driver"`
	NetworkInterface string                      `json:"network_interface"`
	Created          string                      `json:"created"`
	Containers       map[string]NetworkContainer `json:"containers"`
}

func loadNetwork(ctx context.Context, remote bool, name string) (*Network, error) {
	var result Network
	inspectCmd := genCmd(ctx, remote, "network", "inspect", "--format", "json", name)
	output, err := runCmd(inspectCmd)
	if err != nil {
		return nil, fmt.Errorf("can't inspect podman network: %w", err)
	}

	var inspectOutput []NetworkDetails
	if err := json.Unmarshal(output.Bytes(), &inspectOutput); err != nil {
		return nil, fmt.Errorf("can't decode podman network details: %w", err)
	}
	if len(inspectOutput) != 1 {
		return nil, fmt.Errorf("unusual amount of network details: %v", len(inspectOutput))
	}
	result.Name = name
	for _, v := range inspectOutput[0].Containers {
		result.Containers = append(result.Containers, v)
	}
	return &result, nil
}

func (p *PodmanManager) GetNetwork(ctx context.Context, name string) (*Network, error) {
	return loadNetwork(ctx, p.remote, name)
}

func (p *PodmanManager) ListNetworks(ctx context.Context) ([]*Network, error) {
	cmd := genCmd(ctx, p.remote, "network", "ls", "--format", "{{ .Name }}")
	output, err := runCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf("error listing networks: %w", err)
	}

	var networks []*Network
	var names []string
	scanner := bufio.NewScanner(bytes.NewReader(output.Bytes()))
	for scanner.Scan() {
		names = append(names, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading input:", err)
	}

	for _, n := range names {
		net, err := loadNetwork(ctx, p.remote, n)
		if err != nil {
			return nil, fmt.Errorf("can't load network :%w", err)
		}
		networks = append(networks, net)
	}
	return networks, nil
}

func (p *PodmanManager) RemoveNetwork(ctx context.Context, n *Network) error {
	cmd := genCmd(ctx, p.remote, "network", "rm", n.Name)
	_, err := runCmd(cmd)
	if err != nil {
		return err
	}
	return nil
}
