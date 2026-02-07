package executor

import (
	"context"

	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/internal/actions"
)

func installOrUpdateFile(ctx context.Context, e *Executor, v actions.Action) error {
	diffs, err := v.GetContentAsDiffs()
	if err != nil {
		return err
	}
	resourceData := diffmatchpatch.New().DiffText2(diffs)
	if err := e.host.InstallResource(v.Target, []byte(resourceData)); err != nil {
		return err
	}
	return nil
}

func removeFile(ctx context.Context, e *Executor, v actions.Action) error {
	if err := e.host.RemoveResource(v.Target); err != nil {
		return err
	}
	return nil
}

func installDir(ctx context.Context, e *Executor, v actions.Action) error {
	if err := e.host.InstallResource(v.Target, nil); err != nil {
		return err
	}
	return nil
}

func removeDir(ctx context.Context, e *Executor, v actions.Action) error {
	if err := e.host.RemoveResource(v.Target); err != nil {
		return err
	}
	return nil
}
