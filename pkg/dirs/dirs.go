package dirs

import (
	"os"
	"os/user"
	"path/filepath"
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

type MateriaDirs struct {
	Data     string
	Quadlets string
	Services string
	Scripts  string
	Source   string
	Remote   string
	Output   string
}

func DefaultMateriaDirs(u *user.User) (MateriaDirs, error) {
	dataPath := DefaultDataDir
	quadletPath := DefaultQuadletDir
	servicePath := DefaultServiceDir
	scriptsPath := DefaultScriptsDir

	sourcePath := DefaultSourceDir
	outputPath := DefaultOutputDir
	remotePath := DefaultRemoteDir
	if u != nil && u.Uid != "0" {
		home := u.HomeDir
		conf := os.Getenv("XDG_CONFIG_HOME")
		if conf == "" {
			quadletPath = filepath.Join(home, ".config", "containers", "systemd")
		} else {
			quadletPath = filepath.Join(conf, "containers", "systemd")
		}
		datadir := os.Getenv("XDG_DATA_HOME")
		if datadir == "" {
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

	return MateriaDirs{
		Data:     dataPath,
		Quadlets: quadletPath,
		Services: servicePath,
		Scripts:  scriptsPath,
		Source:   sourcePath,
		Remote:   remotePath,
		Output:   outputPath,
	}, nil
}
