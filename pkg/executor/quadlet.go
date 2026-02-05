package executor

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/internal/containers"
	"primamateria.systems/materia/pkg/components"
)

func cleanupNetwork(ctx context.Context, e *Executor, v actions.Action) error {
	network, err := e.host.GetNetwork(ctx, v.Target.HostObject)
	if err != nil {
		return fmt.Errorf("can't cleanup network %v: %w", v.Target, err)
	}
	if len(network.Containers) < 1 {
		err := e.host.RemoveNetwork(ctx, &containers.Network{Name: v.Target.HostObject})
		if err != nil {
			return err
		}
	} else {
		log.Warnf("skipping cleaning up network %v since its still in use", v.Target.Path)
	}
	return nil
}

func cleanupBuildArtifact(ctx context.Context, e *Executor, v actions.Action) error {
	containerWithImage, err := e.host.ListContainers(ctx, containers.ContainerListFilter{
		Image: v.Target.HostObject,
		All:   true,
	})
	if err != nil {
		return fmt.Errorf("can't cleanup image/build %v: %w", v.Target.HostObject, err)
	}
	if len(containerWithImage) > 0 {
		log.Warnf("skipping cleaning up image %v since it's still in use", v.Target.Path)
	} else {
		err = e.host.RemoveImage(ctx, v.Target.HostObject)
		if err != nil {
			return err
		}
	}
	return nil
}

func ensureQuadlet(ctx context.Context, e *Executor, v actions.Action) error {
	err := modifyService(ctx, e.host, actions.Action{
		Todo:   actions.ActionReload,
		Parent: components.NewComponent("root"),
		Target: components.Resource{Kind: components.ResourceTypeHost},
	}, e.defaultTimeout)
	if err != nil {
		return err
	}
	err = modifyService(ctx, e.host, actions.Action{
		Todo:   actions.ActionRestart,
		Parent: v.Parent,
		Target: components.Resource{
			Parent: v.Parent.Name,
			Path:   v.Target.Service(),
			Kind:   components.ResourceTypeService,
		},
	}, e.defaultTimeout)
	if err != nil {
		return err
	}
	return nil
}

func serviceAction(ctx context.Context, e *Executor, v actions.Action) error {
	err := modifyService(ctx, e.host, v, e.defaultTimeout)
	if err != nil {
		return err
	}
	return nil
}
