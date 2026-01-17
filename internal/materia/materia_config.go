package materia

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/internal/attributes/age"
	fileattrs "primamateria.systems/materia/internal/attributes/file"
	"primamateria.systems/materia/internal/attributes/sops"
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
	Debug          bool              `toml:"debug"`
	UseStdout      bool              `toml:"use_stdout"`
	Hostname       string            `toml:"hostname"`
	Roles          []string          `toml:"roles"`
	Timeout        int               `toml:"timeout"`
	MateriaDir     string            `toml:"materia_dir"`
	QuadletDir     string            `toml:"quadlet_dir"`
	ServiceDir     string            `toml:"service_dir"`
	ScriptsDir     string            `toml:"scripts_dir"`
	SourceDir      string            `toml:"source_dir"`
	OutputDir      string            `toml:"output_dir"`
	RemoteDir      string            `toml:"remote_dir"`
	NoSync         bool              `toml:"nosync"`
	OnlyResources  bool              `toml:"only_resources"`
	Quiet          bool              `toml:"quiet"`
	Cleanup        bool              `toml:"cleanup"`
	CleanupVolumes bool              `toml:"cleanup_volumes"`
	BackupVolumes  bool              `toml:"backup_volumes"`
	MigrateVolumes bool              `toml:"migrate_volumes"`
	Attributes     string            `toml:"attributes"`
	CompressionCmd string            `toml:"compression_cmd"`
	SecretsPrefix  string            `toml:"secrets_prefix"`
	AgeConfig      *age.Config       `toml:"age"`
	FileConfig     *fileattrs.Config `toml:"file"`
	SopsConfig     *sops.Config      `toml:"sops"`
	PlannerConfig  *PlannerConfig    `toml:"planner"`
	ExecutorConfig *ExecutorConfig   `toml:"executor"`
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
	c.Timeout = k.Int("timeout")
	if c.Timeout == 0 {
		c.Timeout = 90
	}
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
	if k.Exists("planner") {
		c.PlannerConfig, err = NewPlannerConfig(k)
		if err != nil {
			return nil, err
		}
	} else {
		// TODO remove in 0.6
		pc := &PlannerConfig{}
		if k.Exists("cleanup") || k.Exists("cleanup_volumes") || k.Exists("backup_volumes") || k.Exists("migrate_volumes") {
			log.Warn("configuring planner settings directly is deprecated and will be removed in 0.6")
		}
		pc.CleanupQuadlets = k.Bool("cleanup")
		pc.CleanupVolumes = k.Bool("cleanup_volumes")
		if k.Exists("backup_volumes") {
			pc.BackupVolumes = k.Bool("backup_volumes")
		} else {
			pc.BackupVolumes = true
		}
		pc.MigrateVolumes = k.Bool("migrate_volumes")
		c.PlannerConfig = pc
	}
	c.ExecutorConfig, err = NewExecutorConfig(k)
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

	if c.User.Username != "root" {
		home := c.User.HomeDir
		var found bool
		conf, found := os.LookupEnv("XDG_CONFIG_HOME")
		if !found {
			quadletPath = fmt.Sprintf("%v/.config/containers/systemd/", home)
		} else {
			quadletPath = fmt.Sprintf("%v/containers/systemd/", conf)
		}
		datadir, found := os.LookupEnv("XDG_DATA_HOME")
		if !found {
			dataPath = fmt.Sprintf("%v/.local/share", home)
			servicePath = fmt.Sprintf("%v/.local/share/systemd/user", home)
		} else {
			dataPath = datadir
			servicePath = fmt.Sprintf("%v/systemd/user", datadir)
		}
		scriptsPath = fmt.Sprintf("%v/.local/bin", home)
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
		c.SourceDir = DefaultSourceDir
	}
	if c.RemoteDir == "" {
		c.RemoteDir = DefaultRemoteDir
	}
	if c.OutputDir == "" {
		c.OutputDir = DefaultOutputDir
	}
	c.ExecutorConfig.MateriaDir = c.MateriaDir
	c.ExecutorConfig.QuadletDir = c.QuadletDir
	c.ExecutorConfig.ScriptsDir = c.ScriptsDir
	c.ExecutorConfig.ServiceDir = c.ServiceDir

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
	return nil
}

func (c *MateriaConfig) String() string {
	var result string
	result += fmt.Sprintf("Debug mode: %v\n", c.Debug)
	result += fmt.Sprintf("STDOUT: %v\n", c.UseStdout)
	result += fmt.Sprintf("Cleanup: %v\n", c.Cleanup)
	result += fmt.Sprintf("Configured Hostname: %v\n", c.Hostname)
	result += fmt.Sprintf("Configured Roles: %v\n", c.Roles)
	result += fmt.Sprintf("Service Timeout: %v\n", c.Timeout)
	result += fmt.Sprintf("Materia Root: %v\n", c.MateriaDir)
	result += fmt.Sprintf("Quadlet Dir: %v\n", c.QuadletDir)
	result += fmt.Sprintf("Scripts Dir: %v\n", c.ScriptsDir)
	result += fmt.Sprintf("Source cache dir: %v\n", c.SourceDir)
	result += fmt.Sprintf("Remote cache dir: %v\n", c.RemoteDir)
	result += fmt.Sprintf("Resources Only: %v\n", c.OnlyResources)
	result += fmt.Sprintf("User: %v\n", c.User.Username)
	result += fmt.Sprintf("Remote: %v\n", c.Remote)
	if c.PlannerConfig != nil {
		result += "Planner Config: \n"
		result += fmt.Sprintf("%v", c.PlannerConfig.String())
	}
	if c.ExecutorConfig != nil {
		result += "Executor Config: \n"
		result += fmt.Sprintf("%v", c.ExecutorConfig.String())

	}
	if c.AgeConfig != nil {
		result += "Age Config: \n"
		result += fmt.Sprintf("%v", c.AgeConfig.String())
	}
	if c.FileConfig != nil {
		result += "File Config: \n"
		result += fmt.Sprintf("%v", c.FileConfig.String())
	}
	if c.SopsConfig != nil {
		result += "Sops Config: \n"
		result += fmt.Sprintf("%v", c.SopsConfig.String())
	}

	return result
}
