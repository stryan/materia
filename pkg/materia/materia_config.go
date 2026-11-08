package materia

import (
	"cmp"
	"errors"
	"fmt"
	"os/user"

	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/pkg/attributes/age"
	fileattrs "primamateria.systems/materia/pkg/attributes/file"
	"primamateria.systems/materia/pkg/attributes/sops"
	"primamateria.systems/materia/pkg/containers"
	"primamateria.systems/materia/pkg/dirs"
	"primamateria.systems/materia/pkg/executor"
	"primamateria.systems/materia/pkg/notify"
	"primamateria.systems/materia/pkg/planner"
	"primamateria.systems/materia/pkg/services"
)

type MateriaConfig struct {
	Debug            bool                        `toml:"debug"`
	UseStdout        bool                        `toml:"use_stdout"`
	Hostname         string                      `toml:"hostname"`
	Roles            []string                    `toml:"roles"`
	MateriaDir       string                      `toml:"materia_dir"`
	QuadletDir       string                      `toml:"quadlet_dir"`
	ServiceDir       string                      `toml:"service_dir"`
	ScriptsDir       string                      `toml:"scripts_dir"`
	SourceDir        string                      `toml:"source_dir"`
	OutputDir        string                      `toml:"output_dir"`
	RemoteDir        string                      `toml:"remote_dir"`
	NoSync           bool                        `toml:"nosync"`
	SourceSubpath    string                      `toml:"source_subpath"`
	Lock             string                      `toml:"lock"`
	AppMode          bool                        `toml:"appmode"`
	Rootless         bool                        `toml:"rootless"`
	Attributes       string                      `toml:"attributes"`
	CommandPodman    bool                        `toml:"podman_command"`
	AgeConfig        *age.Config                 `toml:"age"`
	FileConfig       *fileattrs.Config           `toml:"file"`
	SopsConfig       *sops.Config                `toml:"sops"`
	PlannerConfig    planner.PlannerConfig       `toml:"planner"`
	ExecutorConfig   executor.ExecutorConfig     `toml:"executor"`
	ServicesConfig   services.ServicesConfig     `toml:"services"`
	ContainersConfig containers.ContainersConfig `toml:"containers"`
	NotifyConfig     notify.NotifyConfig         `toml:"notify"`
	RollbackConfig   RollbackConfig              `toml:"rollback"`
	User             *user.User
}

func NewConfig(k *koanf.Koanf) (*MateriaConfig, error) {
	c, err := DefaultConfig()
	if err != nil {
		return nil, err
	}
	c.Debug = k.Bool("debug")
	c.UseStdout = k.Bool("use_stdout")

	c.Hostname = k.String("hostname")
	c.Roles = k.Strings("roles")

	// use user specified paths OR the paths already provided via DefaultConfig
	c.SourceDir = cmp.Or(k.String("source_dir"), c.SourceDir)
	c.MateriaDir = cmp.Or(k.String("materia_dir"), c.MateriaDir)
	c.QuadletDir = cmp.Or(k.String("quadlet_dir"), c.QuadletDir)
	c.ServiceDir = cmp.Or(k.String("service_dir"), c.ServiceDir)
	c.ScriptsDir = cmp.Or(k.String("scripts_dir"), c.ScriptsDir)
	c.OutputDir = cmp.Or(k.String("output_dir"), c.OutputDir)
	c.RemoteDir = cmp.Or(k.String("remote_dir"), c.RemoteDir)

	c.NoSync = k.Bool("nosync")
	c.SourceSubpath = k.String("source_subpath")
	c.Lock = k.String("lock")
	c.AppMode = k.Bool("appmode")

	c.Attributes = k.String("attributes")
	if k.Exists("age") || c.Attributes == "age" {
		c.AgeConfig, err = age.NewConfig(k)
		if err != nil {
			return nil, err
		}
	}
	if k.Exists("file") || c.Attributes == "file" {
		c.FileConfig, err = fileattrs.NewConfig(k)
		if err != nil {
			return nil, err
		}
	}
	if k.Exists("sops") || c.Attributes == "sops" {
		c.SopsConfig, err = sops.NewConfig(k)
		if err != nil {
			return nil, err
		}
	}
	c.ServicesConfig, err = services.NewServicesConfig(k)
	if err != nil {
		return nil, err
	}
	c.PlannerConfig, err = planner.NewPlannerConfig(k)
	if err != nil {
		return nil, err
	}

	c.ContainersConfig, err = containers.NewContainersConfig(k)
	if err != nil {
		return nil, err
	}
	c.ExecutorConfig, err = executor.NewExecutorConfig(k)
	if err != nil {
		return nil, err
	}
	c.NotifyConfig, err = notify.NewConfig(k)
	if err != nil {
		return nil, err
	}
	c.RollbackConfig, err = NewRollbackConfig(k)
	if err != nil {
		return nil, err
	}

	c.Rootless = k.Bool("rootless")
	c.CommandPodman = k.Bool("podman_command")

	return c, nil
}

func DefaultConfig() (*MateriaConfig, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	mdirs, err := dirs.DefaultMateriaDirs(currentUser)
	if err != nil {
		return nil, err
	}

	c := &MateriaConfig{
		Debug:            false,
		MateriaDir:       mdirs.Data,
		QuadletDir:       mdirs.Quadlets,
		ServiceDir:       mdirs.Services,
		ScriptsDir:       mdirs.Scripts,
		SourceDir:        mdirs.Source,
		OutputDir:        mdirs.Output,
		RemoteDir:        mdirs.Remote,
		NoSync:           false,
		Lock:             "",
		AppMode:          false,
		Rootless:         false,
		Attributes:       "",
		CommandPodman:    false,
		PlannerConfig:    planner.DefaultPlannerConfig(),
		ExecutorConfig:   executor.ExecutorConfig{},
		ServicesConfig:   services.DefaultServicesConfig(),
		ContainersConfig: containers.DefaultContainersConfig(),
		NotifyConfig:     notify.DefaultNotifyConfig(),
		RollbackConfig:   DefaultRollbackConfig(),
		User:             currentUser,
	}
	return c, nil
}

func (c *MateriaConfig) Validate() error {
	if c.QuadletDir == "" {
		return errors.New("need quadlet directory")
	}
	if c.ServiceDir == "" {
		return errors.New("need services directory")
	}
	if c.ScriptsDir == "" {
		return errors.New("need scripts directory")
	}
	if c.SourceDir == "" {
		return errors.New("need source directory")
	}
	if c.MateriaDir == "" {
		return errors.New("need materia directory")
	}
	if c.OutputDir == "" {
		return errors.New("need output directory")
	}
	if c.RemoteDir == "" {
		return errors.New("need remote directory")
	}
	if err := c.PlannerConfig.Validate(); err != nil {
		return fmt.Errorf("invalid planner config: %w", err)
	}
	if err := c.ExecutorConfig.Validate(); err != nil {
		return fmt.Errorf("invalid executor config: %w", err)
	}
	if err := c.RollbackConfig.Validate(); err != nil {
		return fmt.Errorf("invalid rollback config: %w", err)
	}
	if err := c.ServicesConfig.Validate(); err != nil {
		return fmt.Errorf("invalid services config: %w", err)
	}
	if err := c.ContainersConfig.Validate(); err != nil {
		return fmt.Errorf("invalid containers config: %w", err)
	}
	if err := c.NotifyConfig.Validate(); err != nil {
		return fmt.Errorf("invalid notify config: %w", err)
	}
	return nil
}

func (c *MateriaConfig) String() string {
	var result string
	result += "Materia Config\n"
	result += "\nFile paths\n"
	result += fmt.Sprintf("Materia Root: %v\n", c.MateriaDir)
	result += fmt.Sprintf("Quadlet Dir: %v\n", c.QuadletDir)
	result += fmt.Sprintf("Scripts Dir: %v\n", c.ScriptsDir)
	result += fmt.Sprintf("Systemd Units dir: %v\n", c.ServiceDir)
	result += fmt.Sprintf("Source cache dir: %v\n", c.SourceDir)
	result += fmt.Sprintf("Remote cache dir: %v\n", c.RemoteDir)
	result += fmt.Sprintf("Output Dir: %v\n", c.OutputDir)
	result += "\nGlobal Settings\n"

	result += fmt.Sprintf("Configured Hostname: %v\n", c.Hostname)
	result += fmt.Sprintf("Configured Roles: %v\n", c.Roles)
	if c.User != nil {
		result += fmt.Sprintf("User: %v\n", c.User.Username)
	}
	result += fmt.Sprintf("Debug mode: %v\n", c.Debug)
	result += fmt.Sprintf("Use STDOUT: %v\n", c.UseStdout)
	result += fmt.Sprintf("Sync Source: %v\n", !c.NoSync)
	if c.Lock != "" {
		result += fmt.Sprintf("Lock mode: %v\n", c.Lock)
	} else {
		result += "Lock mode: None\n"
	}
	result += c.RollbackConfig.String()
	result += fmt.Sprintf("Rootless mode: %v\n", c.Rootless)
	result += "\nContainers Config: \n"
	result += fmt.Sprintf("%v", c.ContainersConfig.String())
	result += "\nServices Config: \n"
	result += fmt.Sprintf("%v", c.ServicesConfig.String())
	result += "\nPlanner Config: \n"
	result += fmt.Sprintf("%v", c.PlannerConfig.String())
	result += "\nExecutor Config: \n"
	result += fmt.Sprintf("%v", c.ExecutorConfig.String())
	result += "\nNotify Config: \n"
	result += fmt.Sprintf("%v", c.NotifyConfig.String())
	result += "\nAttributes Config\n"
	if c.Attributes != "" {
		result += fmt.Sprintf("Manually Specified Engine: %v\n", c.Attributes)
	}
	if c.AgeConfig != nil {
		result += "\nAge Config: \n"
		result += fmt.Sprintf("%v", c.AgeConfig.String())
	}
	if c.FileConfig != nil {
		result += "\nFile Config: \n"
		result += fmt.Sprintf("%v", c.FileConfig.String())
	}
	if c.SopsConfig != nil {
		result += "\nSops Config: \n"
		result += fmt.Sprintf("%v", c.SopsConfig.String())
	}

	return result
}
