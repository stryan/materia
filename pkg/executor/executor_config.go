package executor

import (
	"errors"
	"fmt"
	"os/user"
	"path/filepath"

	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/dirs"
)

type ExecutorConfig struct {
	CleanupComponents bool `koanf:"cleanup_components"`
	Verbose           bool `koanf:"verbose"`
	// Overrides
	MateriaDir string `koanf:"materia_dir"`
	QuadletDir string `koanf:"quadlet_dir"`
	ScriptsDir string `koanf:"scripts_dir"`
	ServiceDir string `koanf:"service_dir"`
	OutputDir  string `koanf:"output_dir"`
	// Delegation: only used for components that set User
	DelegateDir     string `koanf:"delegate_dir"`
	DelegateTimeout int    `koanf:"delegate_timeout"`
}

func (e ExecutorConfig) String() string {
	return fmt.Sprintf("Cleanup Components: %v\nMateria Data Dir: %v\nQuadlets Dir: %v\nScripts Dir: %v\nService Dir: %v\nOutput Dir: %v\nDelegate Dir: %v\nDelegate Timeout: %v\n", e.CleanupComponents, e.MateriaDir, e.QuadletDir, e.ScriptsDir, e.ServiceDir, e.OutputDir, e.DelegateDir, e.DelegateTimeout)
}

func NewExecutorConfig(k *koanf.Koanf) (ExecutorConfig, error) {
	ec := ExecutorConfig{}
	if err := k.UnmarshalWithConf("executor", &ec, koanf.UnmarshalConf{}); err != nil {
		return ExecutorConfig{}, fmt.Errorf("unable to create executor config: %w", err)
	}
	return ec, nil
}

func (e ExecutorConfig) Validate() error {
	if e.DelegateTimeout < 0 {
		return errors.New("delegate_timeout can't be negative")
	}
	return nil
}

func DefaultExecutorConfig(user *user.User) (ExecutorConfig, error) {
	dirs, err := dirs.DefaultMateriaDirs(user)
	if err != nil {
		return ExecutorConfig{}, err
	}
	return ExecutorConfig{
		CleanupComponents: false,
		Verbose:           false,
		MateriaDir:        dirs.Data,
		QuadletDir:        dirs.Quadlets,
		ScriptsDir:        dirs.Scripts,
		ServiceDir:        dirs.Services,
		OutputDir:         dirs.Output,
		DelegateDir:       filepath.Join(dirs.Data, "delegate"),
		DelegateTimeout:   DefaultDelegateTimeout,
	}, nil
}
