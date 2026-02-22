package executor

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/log"
	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/pkg/containers"
)

func cleanupVolume(ctx context.Context, e *Executor, v actions.Action) error {
	containersWithVolume, err := e.host.ListContainers(ctx, containers.ContainerListFilter{
		Volume: v.Target.HostObject,
		All:    true,
	})
	if err != nil {
		return fmt.Errorf("can't cleanup volume %v: %w", v.Target, err)
	}
	if len(containersWithVolume) > 0 {
		log.Warnf("skipping cleaning up volume %v since it's still in use", v.Target.Path)
	} else {
		err = e.host.RemoveVolume(ctx, &containers.Volume{Name: v.Target.HostObject})
		if err != nil {
			return err
		}
	}
	return nil
}

func importVolume(ctx context.Context, e *Executor, v actions.Action) error {
	err := e.host.ImportVolume(ctx, &containers.Volume{Name: v.Target.HostObject, Driver: "local"}, filepath.Join(e.OutputDir, fmt.Sprintf("%v.tar", v.Target.HostObject)))
	if err != nil {
		return fmt.Errorf("error importing volume %v: %w", v.Target.HostObject, err)
	}
	return nil
}

func dumpVolume(ctx context.Context, e *Executor, v actions.Action) error {
	err := e.host.DumpVolume(ctx, &containers.Volume{Name: v.Target.HostObject}, e.OutputDir, false)
	if err != nil {
		return fmt.Errorf("error dumping volume %v:%w", v.Target.Path, err)
	}
	return nil
}
