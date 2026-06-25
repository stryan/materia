package executor

import (
	"errors"
	"fmt"

	"github.com/knadh/koanf/v2"
)

type ExecutorConfig struct {
	CleanupComponents bool   `koanf:"cleanup_components"`
	MateriaDir        string `koanf:"materia_dir"`
	QuadletDir        string `koanf:"quadlet_dir"`
	ScriptsDir        string `koanf:"scripts_dir"`
	ServiceDir        string `koanf:"service_dir"`
	OutputDir         string `koanf:"output_dir"`
}

func (e *ExecutorConfig) String() string {
	return fmt.Sprintf("Cleanup Components: %v\nMateria Data Dir: %v\nQuadlets Dir: %v\nScripts Dir: %v\nService Dir: %v\n", e.CleanupComponents, e.MateriaDir, e.QuadletDir, e.ScriptsDir, e.ServiceDir)
}

func NewExecutorConfig(k *koanf.Koanf) (*ExecutorConfig, error) {
	ec := &ExecutorConfig{}
	err := k.UnmarshalWithConf("executor", ec, koanf.UnmarshalConf{})
	return ec, err
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
