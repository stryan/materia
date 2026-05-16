package native

import (
	"context"
	"errors"

	nw "go.podman.io/podman/v6/pkg/bindings/network"
	"primamateria.systems/materia/pkg/containers"
)

func (n *NativeManager) GetNetwork(ctx context.Context, name string) (*containers.Network, error) {
	net, err := nw.Inspect(n.conn, name, nil)
	if err != nil {
		return nil, err
	}
	result := &containers.Network{
		Name: name,
	}
	for _, c := range net.Containers {
		result.Containers = append(result.Containers, containers.NetworkContainer{
			Name: c.Name,
		})
	}
	return result, nil
}

func (n *NativeManager) ListNetworks(ctx context.Context) ([]*containers.Network, error) {
	networks, err := nw.List(n.conn, nil)
	if err != nil {
		return nil, err
	}
	result := make([]*containers.Network, 0, len(networks))
	for _, net := range networks {
		entry, err := n.GetNetwork(ctx, net.Name)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, nil
}

func (n *NativeManager) RemoveNetwork(ctx context.Context, network *containers.Network) error {
	if network == nil {
		return errors.New("tried to remove nil network")
	}
	_, err := nw.Remove(n.conn, network.Name, &nw.RemoveOptions{})
	return err
}
