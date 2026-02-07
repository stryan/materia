package executor

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/sergi/go-diff/diffmatchpatch"
	"primamateria.systems/materia/internal/actions"
	"primamateria.systems/materia/internal/services"
)

func installOrUpdateScript(ctx context.Context, e *Executor, v actions.Action) error {
	diffs, err := v.GetContentAsDiffs()
	if err != nil {
		return err
	}
	resourceData := diffmatchpatch.New().DiffText2(diffs)
	if err := e.host.InstallResource(v.Target, []byte(resourceData)); err != nil {
		return err
	}
	return e.host.InstallScript(ctx, v.Target.Path, []byte(resourceData))
}

func removeScript(ctx context.Context, e *Executor, v actions.Action) error {
	if err := e.host.RemoveResource(v.Target); err != nil {
		return err
	}
	if err := e.host.RemoveScript(ctx, v.Target.Path); err != nil {
		return err
	}
	return nil
}

func setupScript(ctx context.Context, e *Executor, v actions.Action) error {
	scriptPath := filepath.Join(e.ScriptsDir, v.Target.Path)
	setupName := fmt.Sprintf("%v-materia-setup.service", v.Parent.Name)
	cleanupName := fmt.Sprintf("%v-materia-cleanup.service", v.Parent.Name)
	if err := e.host.RunOneshotCommand(ctx, e.defaultTimeout, setupName, []string{scriptPath}); err != nil {
		return err
	}
	// we succesfully setup, remove any cleanup script instances
	if err := e.host.Apply(ctx, cleanupName, services.ServiceStop, e.defaultTimeout); err != nil {
		log.Warnf("couldn't remove old cleanup script instance for %v: %v", v.Parent.Name, err)
	}
	return nil
}

func cleanupScript(ctx context.Context, e *Executor, v actions.Action) error {
	scriptPath := filepath.Join(e.ScriptsDir, v.Target.Path)
	setupName := fmt.Sprintf("%v-materia-setup.service", v.Parent.Name)
	cleanupName := fmt.Sprintf("%v-materia-cleanup.service", v.Parent.Name)
	if err := e.host.RunOneshotCommand(ctx, e.defaultTimeout, cleanupName, []string{scriptPath}); err != nil {
		return err
	}
	// we succesfully setup, remove any setup script instances
	if err := e.host.Apply(ctx, setupName, services.ServiceStop, e.defaultTimeout); err != nil {
		log.Warnf("couldn't remove old cleanup script instance for %v: %v", v.Parent.Name, err)
	}
	return nil
}
