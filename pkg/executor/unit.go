package executor

import (
	"bytes"
	"context"

	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/internal/actions"
)

func installOrUpdateUnit(ctx context.Context, e *Executor, v actions.Action) error {
	diffs, err := v.GetContentAsDiffs()
	if err != nil {
		return err
	}
	resourceData := bytes.NewBufferString(diffmatchpatch.New().DiffText2(diffs))
	if err := e.host.InstallResource(v.Target, resourceData); err != nil {
		return err
	}
	if err := e.host.InstallUnit(ctx, v.Target.Path, resourceData); err != nil {
		return err
	}
	return nil
}

func removeUnit(ctx context.Context, e *Executor, v actions.Action) error {
	if err := e.host.RemoveResource(v.Target); err != nil {
		return err
	}
	if err := e.host.RemoveUnit(ctx, v.Target.Path); err != nil {
		return err
	}
	return nil
}
