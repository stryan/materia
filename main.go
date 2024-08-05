package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"git.saintnet.tech/stryan/materia/internal/secrets"
	"git.saintnet.tech/stryan/materia/internal/secrets/mem"
	"git.saintnet.tech/stryan/materia/internal/source/git"
	"github.com/charmbracelet/log"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/kelseyhightower/envconfig"
	"github.com/nikolalohinski/gonja/v2"
)

type Config struct {
	GitRepo string
}

func main() {
	var c Config
	err := envconfig.Process("materia", &c)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Default().SetLevel(log.DebugLevel)
	currentUser, err := user.Current()
	if err != nil {
		log.Fatalf(err.Error())
	}
	ctx := context.Background()
	prefix := "/var/lib"
	destination := "/etc/systemd/system"
	if currentUser.Username != "root" {
		home := currentUser.HomeDir
		var found bool

		conf, found := os.LookupEnv("XDG_CONFIG_HOME")
		if !found {
			destination = fmt.Sprintf("%v/.config/containers/systemd/", home)
		} else {
			destination = fmt.Sprintf("%v/containers/systemd/", conf)
		}
		prefix, found = os.LookupEnv("XDG_DATA_HOME")
		if !found {
			prefix = fmt.Sprintf("%v/.local/share", home)
		}
	}
	m := NewMateria(prefix, destination)
	err = m.SetupHost()
	if err != nil {
		log.Fatal(err)
	}
	prefix = fmt.Sprintf("%v/materia", prefix)
	source := git.NewGitSource(fmt.Sprintf("%v/source", prefix), c.GitRepo)
	err = source.Sync(ctx)
	if err != nil {
		log.Fatal(err)
	}
	sm := mem.NewMemoryManager()
	sm.Add("container_tag", "latest")
	sm.Add("hello_config", "HI!")
	decans, err := determineDecans(m)
	if err != nil {
		log.Fatal(err)
	}
	applied, err := applyDecans(m, sm, decans)
	if err != nil {
		log.Fatal(err)
	}
	var conn *dbus.Conn
	if currentUser.Username != "root" {
		conn, err = dbus.NewUserConnectionContext(ctx)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		conn, err = dbus.NewSystemConnectionContext(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}
	defer conn.Close()
	for _, v := range applied {
		// first scan for reload requirements
		if v.NeedReload {
			err = conn.ReloadContext(ctx)
			if err != nil {
				log.Fatal(err)
			}
			break // we only need it once
		}
	}
	// start/restart services
	for _, v := range applied {
		// now restart services
		if len(v.RestartServices) > 0 {
			callback := make(chan string)
			for _, unit := range v.RestartServices {
				log.Info("restarting service %v", unit)
				_, err := conn.ReloadOrTryRestartUnitContext(ctx, unit, "replace", callback)
				if err != nil {
					log.Warn(err)
				}
				<-callback
			}
		}
		if len(v.NewServices) > 0 {
			callback := make(chan string)
			for _, unit := range v.NewServices {
				log.Info("restarting service %v", unit)
				_, err := conn.StartUnitContext(ctx, unit, "fail", callback)
				if err != nil {
					log.Warn(err)
				}
				<-callback
			}
		}
	}
}

func determineDecans(m *Materia) ([]*Decan, error) {
	var decans []*Decan
	// TODO: map decans to host, for now we just apply all of them
	entries, err := os.ReadDir(m.AllDecanSourcePaths())
	if err != nil {
		return decans, err
	}
	var decanPaths []string
	for _, v := range entries {
		log.Debug("possible decan", "entry", v.Name())
		if v.IsDir() {
			decanPaths = append(decanPaths, v.Name())
		}
	}
	for _, v := range decanPaths {
		decans = append(decans, NewDecan(filepath.Join(m.AllDecanSourcePaths(), v)))
	}

	return decans, nil
}

type applicationResult struct {
	NeedReload      bool
	RestartServices []string
	NewServices     []string
}

func applyDecans(m *Materia, sm secrets.SecretsManager, decans []*Decan) ([]applicationResult, error) {
	var results []applicationResult
	secretContext := sm.All(context.Background())
	for _, v := range decans {
		log.Infof("applying %v", v.Name)
		err := os.Mkdir(m.DecanInstallPath(v), 0o755)
		if err != nil && os.IsNotExist(err) {
			return results, err
		}
		application := applicationResult{}

		for _, star := range v.resources {
			var result []byte
			data, err := os.ReadFile(star.Path)
			if err != nil {
				return results, err
			}
			if star.Template {
				log.Debug("applying template", "file", star.Name)
				tmpl, err := gonja.FromBytes(data)
				if err != nil {
					return results, err
				}
				result, err = tmpl.ExecuteToBytes(secretContext)
				if err != nil {
					return results, err
				}
			} else {
				result = data
			}
			finalDest := m.DecanInstallPath(v)
			if star.Quadlet {
				finalDest = m.InstallPath()
			}
			finalpath := filepath.Join(finalDest, star.Name)
			newFile := true
			if _, err := os.Stat(finalpath); !os.IsNotExist(err) {
				// file already exists, lets see if we're actually making changes
				// TODO: read and compare by chunks
				existingData, err := os.ReadFile(finalpath)
				if err != nil {
					return results, err
				}
				if bytes.Equal(result, existingData) {
					// no diff, skip file
					log.Debug("skipping existing unchanged file", "filename", star.Name, "destination", finalDest)
					continue
				} else {
					newFile = false
				}
			}
			log.Debug("writing file", "filename", star.Name, "destination", finalDest)
			err = os.WriteFile(filepath.Join(finalDest, star.Name), result, 0o755)
			if err != nil {
				return results, err
			}
			if star.Quadlet {
				application.NeedReload = true
			} else {
				if newFile {
					application.NewServices = append(application.NewServices, v.ServiceForResource(star)...)
				} else {
					application.RestartServices = append(application.RestartServices, v.ServiceForResource(star)...)
				}
			}
		}
		results = append(results, application)

	}
	return results, nil
}
