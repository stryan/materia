package executor

import (
	"context"

	"primamateria.systems/materia/internal/actions"
)

func installComponent(ctx context.Context, e *Executor, v actions.Action) error {
	return e.host.InstallComponent(v.Parent)
}

func updateComponent(ctx context.Context, e *Executor, v actions.Action) error {
	return e.host.UpdateComponent(v.Parent)
}

func removeComponent(ctx context.Context, e *Executor, v actions.Action) error {
	return e.host.RemoveComponent(v.Parent)
}
