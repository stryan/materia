package native

import (
	"context"
	"fmt"

	"github.com/moby/moby/api/types/container"
	"go.podman.io/podman/v6/pkg/api/handlers"
	podman "go.podman.io/podman/v6/pkg/bindings/containers"
	"primamateria.systems/materia/pkg/containers"
)

func (n *NativeManager) GetContainer(_ context.Context, name string) (*containers.Container, error) {
	fullContainer, err := podman.Inspect(n.conn, name, &podman.InspectOptions{})
	if err != nil {
		return nil, err
	}
	result := &containers.Container{
		Name:       name,
		Id:         fullContainer.ID,
		Hostname:   fullContainer.Config.Hostname,
		Volumes:    map[string]containers.Volume{},
		BindMounts: map[string]containers.ContainerMount{},
	}
	for _, v := range fullContainer.Mounts {
		switch v.Type {
		case "bind":
			result.BindMounts[v.Destination] = containers.ContainerMount{
				Type:        "bind",
				Name:        v.Destination,
				Source:      v.Source,
				Destination: v.Destination,
			}
		case "volume":
			result.Volumes[v.Name] = containers.Volume{
				Name:       v.Name,
				Mountpoint: v.Source,
				Driver:     v.Driver,
			}
		default:
			continue
		}
	}
	return result, nil
}

func (n *NativeManager) ListContainers(ctx context.Context, filter containers.ContainerListFilter) ([]*containers.Container, error) {
	opts := &podman.ListOptions{
		Filters: make(map[string][]string),
	}
	if filter.All {
		opts.All = &filter.All
	}
	if filter.Image != "" {
		opts.Filters["image"] = []string{filter.Image}
	}
	if filter.Network != "" {
		opts.Filters["network"] = []string{filter.Network}
	}
	if filter.Pod != "" {
		opts.Filters["pod"] = []string{filter.Pod}
	}
	if filter.Volume != "" {
		opts.Filters["volume"] = []string{filter.Volume}
	}

	entries, err := podman.List(n.conn, opts)
	if err != nil {
		return nil, err
	}
	result := make([]*containers.Container, 0, len(entries))
	for _, v := range entries {
		if len(v.Names) == 0 {
			// we probably don't care about this container
			continue
		}
		fetched, err := n.GetContainer(ctx, v.Names[0])
		if err != nil {
			return nil, err
		}
		result = append(result, fetched)
	}
	return result, nil
}

func (n *NativeManager) ExecContainer(_ context.Context, name string, args ...string) error {
	session, err := podman.ExecCreate(n.conn, name, &handlers.ExecCreateConfig{
		ExecCreateRequest: container.ExecCreateRequest{
			Cmd: args,
		},
	})
	if err != nil {
		return fmt.Errorf("unable to create podman exec session: %w", err)
	}

	if err := podman.ExecStartAndAttach(n.conn, session, &podman.ExecStartAndAttachOptions{}); err != nil {
		return fmt.Errorf("unable to podman exec: %w", err)
	}

	result, err := podman.ExecInspect(n.conn, session, nil)
	if err != nil {
		return fmt.Errorf("inspecting exec session: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("%w: command [%v], ec: %d", containers.ErrPodmanExecFailure, args, result.ExitCode)
	}
	return nil
}

func (n *NativeManager) PauseContainer(_ context.Context, name string) error {
	return podman.Pause(n.conn, name, nil)
}

func (n *NativeManager) UnpauseContainer(_ context.Context, name string) error {
	return podman.Unpause(n.conn, name, nil)
}
