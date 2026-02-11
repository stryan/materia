package executor

import (
	"errors"
	"fmt"

	"github.com/knadh/koanf/v2"
)

type ExecutorConfig struct {
	CleanupComponents bool   `toml:"cleanup_components"`
	MateriaDir        string `toml:"materia_dir"`
	QuadletDir        string `toml:"quadlet_dir"`
	ScriptsDir        string `toml:"scripts_dir"`
	ServiceDir        string `toml:"service_dir"`
	OutputDir         string `toml:"output_dir"`
}

func (e *ExecutorConfig) String() string {
	return fmt.Sprintf("Cleanup Components: %v\nMateria Data Dir: %v\nQuadlets Dir: %v\nScripts Dir: %v\nService Dir: %v\n", e.CleanupComponents, e.MateriaDir, e.QuadletDir, e.ScriptsDir, e.ServiceDir)
}

func NewExecutorConfig(k *koanf.Koanf) (*ExecutorConfig, error) {
	ec := &ExecutorConfig{
		CleanupComponents: k.Bool("executor.cleanup_components"),
		MateriaDir:        k.String("executor.materia_dir"),
		QuadletDir:        k.String("executor.quadlet_dir"),
		ScriptsDir:        k.String("executor.scripts_dir"),
		ServiceDir:        k.String("executor.service_dir"),
		OutputDir:         k.String("executor.output_dir"),
	}

	return ec, nil
}

func (e *ExecutorConfig) Validate() error {
	if e.MateriaDir == "" {
		return errors.New("no materia directory set")
	}
	if e.QuadletDir == "" {
		return errors.New("no quadlet directory set")
	}
	if e.ScriptsDir == "" {
		return errors.New("no scripts directory set")
	}
	if e.ServiceDir == "" {
		return errors.New("no service directory set")
	}
	if e.OutputDir == "" {
		return errors.New("no output directory set")
	}
	return nil
}
