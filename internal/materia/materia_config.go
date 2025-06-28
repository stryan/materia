package materia

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/internal/secrets/age"
	filesecrets "primamateria.systems/materia/internal/secrets/file"
)

type MateriaConfig struct {
	Debug         bool
	UseStdout     bool
	Diffs         bool
	Cleanup       bool
	Hostname      string
	Roles         []string
	Timeout       int
	MateriaDir    string
	QuadletDir    string
	ServiceDir    string
	ScriptsDir    string
	SourceDir     string
	OutputDir     string
	OnlyResources bool
	Quiet         bool
	AgeConfig     *age.Config
	FileConfig    *filesecrets.Config
	User          *user.User
}

// var defaultConfig = map[string]any{
// 	"debug":      "",
// 	"prefix":     "",
// 	"quadletdir": "",
// 	"servicedir": "",
// 	"scriptsdir": "",
// }

func NewConfig(k *koanf.Koanf, cliflags map[string]any) (*MateriaConfig, error) {
	var c MateriaConfig
	var err error
	c.SourceDir = k.String("sourcedir")
	c.Debug = k.Bool("debug")
	c.Cleanup = k.Bool("cleanup")
	c.Hostname = k.String("hostname")
	c.Timeout = k.Int("timeout")
	c.Roles = k.Strings("roles")
	c.Diffs = k.Bool("diffs")
	c.UseStdout = k.Bool("stdout")
	c.MateriaDir = k.String("prefix")
	c.QuadletDir = k.String("quadletdir")
	c.ServiceDir = k.String("servicedir")
	c.ScriptsDir = k.String("scriptsdir")
	c.OutputDir = k.String("outputdir")
	if k.Exists("age") {
		c.AgeConfig, err = age.NewConfig(k.Cut("age"))
		if err != nil {
			return nil, err
		}
	}
	if k.Exists("file") {
		c.FileConfig, err = filesecrets.NewConfig(k.Cut("file"))
		if err != nil {
			return nil, err
		}
	}
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	c.User = currentUser

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
	if c.OutputDir == "" {
		c.OutputDir = filepath.Join(dataPath, "materia", "output")
	}

	// apply cli flags
	err = k.Load(confmap.Provider(cliflags, "."), nil)
	if err != nil {
		return nil, err
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
	result += fmt.Sprintf("Cleanup: %v\n", c.Cleanup)
	result += fmt.Sprintf("Hostname: %v\n", c.Hostname)
	result += fmt.Sprintf("Configured Roles: %v\n", c.Roles)
	result += fmt.Sprintf("Service Timeout: %v\n", c.Timeout)
	result += fmt.Sprintf("Materia Root: %v\n", c.MateriaDir)
	result += fmt.Sprintf("Quadlet Dir: %v\n", c.QuadletDir)
	result += fmt.Sprintf("Scripts Dir: %v\n", c.ScriptsDir)
	result += fmt.Sprintf("Source cache dir: %v\n", c.SourceDir)
	result += fmt.Sprintf("User: %v\n", c.User.Username)
	return result
}
