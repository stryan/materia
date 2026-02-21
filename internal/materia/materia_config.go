package materia

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/internal/attributes/age"
	fileattrs "primamateria.systems/materia/internal/attributes/file"
	"primamateria.systems/materia/internal/attributes/sops"
	"primamateria.systems/materia/internal/services"
	"primamateria.systems/materia/pkg/executor"
	"primamateria.systems/materia/pkg/planner"
)

var (
	DefaultDataDir    = "/var/lib/materia"
	DefaultQuadletDir = "/etc/containers/systemd"
	// TODO once we can determine whether /var and /root are on the same filesystem switch this to a /var/lib/materia path and systemctl-link them in
	// otherwise, defer to usual /etc location to work out of the box with MicroOS
	DefaultServiceDir = "/etc/systemd/system"
	DefaultScriptsDir = "/usr/local/bin"

	DefaultSourceDir = filepath.Join(DefaultDataDir, "source")
	DefaultRemoteDir = filepath.Join(DefaultDataDir, "remote")
	DefaultOutputDir = filepath.Join(DefaultDataDir, "output")
)

type MateriaConfig struct {
	Debug          bool                     `toml:"debug"`
	UseStdout      bool                     `toml:"use_stdout"`
	Hostname       string                   `toml:"hostname"`
	Roles          []string                 `toml:"roles"`
	MateriaDir     string                   `toml:"materia_dir"`
	QuadletDir     string                   `toml:"quadlet_dir"`
	ServiceDir     string                   `toml:"service_dir"`
	ScriptsDir     string                   `toml:"scripts_dir"`
	SourceDir      string                   `toml:"source_dir"`
	OutputDir      string                   `toml:"output_dir"`
	RemoteDir      string                   `toml:"remote_dir"`
	NoSync         bool                     `toml:"nosync"`
	OnlyResources  bool                     `toml:"only_resources"`
	Quiet          bool                     `toml:"quiet"`
	AppMode        bool                     `toml:"appmode"`
	Cleanup        bool                     `toml:"cleanup"`
	CleanupVolumes bool                     `toml:"cleanup_volumes"`
	BackupVolumes  bool                     `toml:"backup_volumes"`
	MigrateVolumes bool                     `toml:"migrate_volumes"`
	Attributes     string                   `toml:"attributes"`
	CompressionCmd string                   `toml:"compression_cmd"`
	SecretsPrefix  string                   `toml:"secrets_prefix"`
	AgeConfig      *age.Config              `toml:"age"`
	FileConfig     *fileattrs.Config        `toml:"file"`
	SopsConfig     *sops.Config             `toml:"sops"`
	PlannerConfig  *planner.PlannerConfig   `toml:"planner"`
	ExecutorConfig *executor.ExecutorConfig `toml:"executor"`
	ServicesConfig *services.ServicesConfig `toml:"services"`
	User           *user.User
	Remote         bool `toml:"remote"`
	Rootless       bool `toml:"rootless"`
}

func NewConfig(k *koanf.Koanf) (*MateriaConfig, error) {
	var c MateriaConfig
	var err error
	c.SourceDir = k.String("source_dir")
	c.Debug = k.Bool("debug")
	c.Hostname = k.String("hostname")
	c.Roles = k.Strings("roles")

	c.Attributes = k.String("attributes")
	c.UseStdout = k.Bool("use_stdout")
	c.MateriaDir = k.String("materia_dir")
	c.QuadletDir = k.String("quadlet_dir")
	c.ServiceDir = k.String("service_dir")
	c.ScriptsDir = k.String("scripts_dir")
	c.OutputDir = k.String("output_dir")
	c.RemoteDir = k.String("remote_dir")
	c.NoSync = k.Bool("nosync")
	c.SecretsPrefix = k.String("secrets_prefix")
	c.AppMode = k.Bool("appmode")
	if c.SecretsPrefix == "" {
		c.SecretsPrefix = "materia-"
	}
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
	var servicesCfg services.ServicesConfig
	if k.Exists("services") {
		servicesCfg.DryrunQuadlets = k.Bool("services.dryrun_quadlets")
		servicesCfg.Timeout = k.Int("services.timeout")
	} else {
		// TODO remove in 0.7
		servicesCfg = services.ServicesConfig{
			Timeout: k.Int("timeout"),
		}
	}
	if servicesCfg.Timeout == 0 {
		servicesCfg.Timeout = 90
	}
	c.ServicesConfig = &servicesCfg
	if k.Exists("planner") {
		c.PlannerConfig, err = planner.NewPlannerConfig(k)
		if err != nil {
			return nil, err
		}
	} else {
		c.PlannerConfig = planner.DefaultPlannerConfig()
	}
	c.ExecutorConfig, err = executor.NewExecutorConfig(k)
	if err != nil {
		return nil, err
	}
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	c.User = currentUser
	if k.Exists("compression_cmd") {
		c.CompressionCmd = k.String("compression_cmd")
	} else {
		c.CompressionCmd = "zstd"
	}
	if k.Exists("remote") {
		c.Remote = k.Bool("remote")
	} else {
		c.Remote = (os.Getenv("container") == "podman")
	}
	c.Rootless = k.Bool("rootless")

	dataPath := DefaultDataDir
	quadletPath := DefaultQuadletDir
	servicePath := DefaultServiceDir
	scriptsPath := DefaultScriptsDir

	sourcePath := DefaultSourceDir
	outputPath := DefaultOutputDir
	remotePath := DefaultRemoteDir

	if c.User.Username != "root" {
		home := c.User.HomeDir
		var found bool
		conf, found := os.LookupEnv("XDG_CONFIG_HOME")
		if !found {
			quadletPath = filepath.Join(home, ".config", "containers", "systemd")
		} else {
			quadletPath = filepath.Join(conf, "containers", "systemd")
		}
		datadir, found := os.LookupEnv("XDG_DATA_HOME")
		if !found {
			dataPath = filepath.Join(home, ".local", "share")
			servicePath = filepath.Join(home, ".local", "share", "systemd", "user")
		} else {
			dataPath = datadir
			servicePath = filepath.Join(datadir, "systemd", "user")
		}
		dataPath = filepath.Join(dataPath, "materia")
		scriptsPath = filepath.Join(home, ".local", "bin")
		sourcePath = filepath.Join(dataPath, "source")
		outputPath = filepath.Join(dataPath, "output")
		remotePath = filepath.Join(dataPath, "remote")
	}
	if c.MateriaDir == "" {
		c.MateriaDir = dataPath
	}
	if c.QuadletDir == "" {
		c.QuadletDir = quadletPath
	}
	if c.ServiceDir == "" {
		c.ServiceDir = servicePath
	}
	if c.ScriptsDir == "" {
		c.ScriptsDir = scriptsPath
	}
	if c.SourceDir == "" {
		c.SourceDir = sourcePath
	}
	if c.RemoteDir == "" {
		c.RemoteDir = remotePath
	}
	if c.OutputDir == "" {
		c.OutputDir = outputPath
	}
	c.ExecutorConfig.MateriaDir = c.MateriaDir
	c.ExecutorConfig.QuadletDir = c.QuadletDir
	c.ExecutorConfig.ScriptsDir = c.ScriptsDir
	c.ExecutorConfig.ServiceDir = c.ServiceDir
	c.ExecutorConfig.OutputDir = c.OutputDir

	return &c, nil
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
	if c.PlannerConfig != nil {
		if err := c.PlannerConfig.Validate(); err != nil {
			return fmt.Errorf("invalid planner config: %w", err)
		}
	}
	if c.ExecutorConfig != nil {
		if err := c.ExecutorConfig.Validate(); err != nil {
			return fmt.Errorf("invalid executor config: %w", err)
		}
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
	result += fmt.Sprintf("User: %v\n", c.User.Username)
	result += fmt.Sprintf("Debug mode: %v\n", c.Debug)
	result += fmt.Sprintf("Use STDOUT: %v\n", c.UseStdout)
	result += fmt.Sprintf("Podman Secrets Prefix: %v\n", c.SecretsPrefix)
	result += fmt.Sprintf("Sync Source: %v\n", !c.NoSync)
	result += fmt.Sprintf("Quiet mode: %v\n", c.Quiet)
	result += fmt.Sprintf("Resources Changes Only: %v\n", c.OnlyResources)
	result += fmt.Sprintf("Remote mode: %v\n", c.Remote)
	result += fmt.Sprintf("Rootless mode: %v\n", c.Rootless)
	if c.ServicesConfig != nil {
		result += "\nServices Config: \n"
		result += fmt.Sprintf("%v", c.ServicesConfig.String())
	}
	if c.PlannerConfig != nil {
		result += "\nPlanner Config: \n"
		result += fmt.Sprintf("%v", c.PlannerConfig.String())
	}
	if c.ExecutorConfig != nil {
		result += "\nExecutor Config: \n"
		result += fmt.Sprintf("%v", c.ExecutorConfig.String())
	}
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
