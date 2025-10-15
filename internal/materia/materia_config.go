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
)

type MateriaConfig struct {
	Debug          bool              `toml:"debug"`
	UseStdout      bool              `toml:"use_stdout"`
	Diffs          bool              `toml:"diffs"`
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
	OnlyResources  bool              `toml:"only_resources"`
	Quiet          bool              `toml:"quiet"`
	Cleanup        bool              `toml:"cleanup"`
	CleanupVolumes bool              `toml:"cleanup_volumes"`
	BackupVolumes  bool              `toml:"backup_volumes"`
	MigrateVolumes bool              `toml:"migrate_volumes"`
	Attributes     string            `toml:"attributes"`
	CompressionCmd string            `toml:"compression_cmd"`
	AgeConfig      *age.Config       `toml:"age"`
	FileConfig     *fileattrs.Config `toml:"file"`
	SopsConfig     *sops.Config      `toml:"sops"`
	User           *user.User
}

// var defaultConfig = map[string]any{
// 	"debug":      "",
// 	"prefix":     "",
// 	"quadletdir": "",
// 	"servicedir": "",
// 	"scriptsdir": "",
// }

func NewConfig(k *koanf.Koanf) (*MateriaConfig, error) {
	var c MateriaConfig
	var err error
	c.SourceDir = k.String("source_dir")
	c.Debug = k.Bool("debug")
	c.Cleanup = k.Bool("cleanup")
	c.Hostname = k.String("hostname")
	c.Timeout = k.Int("timeout")
	c.Roles = k.Strings("roles")
	c.Diffs = k.Bool("diffs")
	c.CleanupVolumes = k.Bool("cleanup_volumes")
	if k.Exists("backup_volumes") {
		c.BackupVolumes = k.Bool("backup_volumes")
	} else {
		c.BackupVolumes = true
	}
	c.MigrateVolumes = k.Bool("migrate_volumes")
	c.Attributes = k.String("attributes")
	c.UseStdout = k.Bool("use_stdout")
	c.MateriaDir = k.String("materia_dir")
	c.QuadletDir = k.String("quadlet_dir")
	c.ServiceDir = k.String("service_dir")
	c.ScriptsDir = k.String("scripts_dir")
	c.OutputDir = k.String("output_dir")
	c.RemoteDir = k.String("remote_dir")
	if k.Exists("age") {
		c.AgeConfig, err = age.NewConfig(k)
		if err != nil {
			return nil, err
		}
	}
	if k.Exists("file") {
		c.FileConfig, err = fileattrs.NewConfig(k)
		if err != nil {
			return nil, err
		}
	}
	if k.Exists("sops") {
		c.SopsConfig, err = sops.NewConfig(k)
		if err != nil {
			return nil, err
		}
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

	// calculate defaults
	dataPath := "/var/lib"
	quadletPath := "/etc/containers/systemd/"
	// TODO once we can determine whether /var and /root are on the same filesystem switch this to a /var/lib/materia path and systemctl-link them in
	// otherwise, defer to usual /etc location to work out of the box with MicroOS
	servicePath := "/etc/systemd/system/"
	scriptsPath := "/usr/local/bin"

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
		c.SourceDir = filepath.Join(dataPath, "materia", "source")
	}
	if c.RemoteDir == "" {
		c.RemoteDir = filepath.Join(dataPath, "materia", "remote")
	}
	if c.OutputDir == "" {
		c.OutputDir = filepath.Join(dataPath, "materia", "output")
	}

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
	result += fmt.Sprintf("Show Diffs: %v\n", c.Diffs)
	result += fmt.Sprintf("Clean-up Volumes: %v\n", c.CleanupVolumes)
	result += fmt.Sprintf("Back-up Volumes: %v\n", c.BackupVolumes)
	result += fmt.Sprintf("Migrate Volumes: %v\n", c.MigrateVolumes)
	result += fmt.Sprintf("Cleanup: %v\n", c.Cleanup)
	result += fmt.Sprintf("Hostname: %v\n", c.Hostname)
	result += fmt.Sprintf("Configured Roles: %v\n", c.Roles)
	result += fmt.Sprintf("Service Timeout: %v\n", c.Timeout)
	result += fmt.Sprintf("Materia Root: %v\n", c.MateriaDir)
	result += fmt.Sprintf("Quadlet Dir: %v\n", c.QuadletDir)
	result += fmt.Sprintf("Scripts Dir: %v\n", c.ScriptsDir)
	result += fmt.Sprintf("Source cache dir: %v\n", c.SourceDir)
	result += fmt.Sprintf("Remote cache dir: %v\n", c.RemoteDir)
	result += fmt.Sprintf("Resources Only: %v\n", c.OnlyResources)
	result += fmt.Sprintf("User: %v\n", c.User.Username)
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
