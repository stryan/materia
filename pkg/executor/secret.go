package executor

import (
	"context"

	"primamateria.systems/materia/internal/actions"
)

func installOrUpdateSecret(ctx context.Context, e *Executor, v actions.Action) error {
	if err := e.host.WriteSecret(ctx, v.Target.Path, v.Target.Content); err != nil {
		return err
	}
	return nil
}

func removeSecret(ctx context.Context, e *Executor, v actions.Action) error {
	if err := e.host.RemoveSecret(ctx, v.Target.Path); err != nil {
		return err
	}
	return nil
}
