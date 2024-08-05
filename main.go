package main

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"time"

	"git.saintnet.tech/stryan/materia/internal/secrets/mem"
	"git.saintnet.tech/stryan/materia/internal/source/git"
	"github.com/charmbracelet/log"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	GitRepo string
	Timeout int
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
	state := "/var/lib"
	destination := "/etc/systemd/system"
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30
	}
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
		state, found = os.LookupEnv("XDG_DATA_STATE")
		if !found {
			state = fmt.Sprintf("%v/.local/state", home)
		}
	}
	m := NewMateria(prefix, destination, state)
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
	actions, err := m.DetermineDecans()
	if err != nil {
		log.Fatal(err)
	}
	var results []applicationResult
	for _, v := range actions {
		var res []applicationResult
		if v.Enabled {
			log.Info("applying decan", "decan", v.Name)
			res, err = m.ApplyDecan(v.Name, sm)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Info("removing decan", "decan", v.Name)
			res, err = m.RemoveDecan(v.Name)
			if err != nil {
				log.Fatal(err)
			}
		}
		results = append(results, res...)
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
	for _, v := range results {
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
	for _, v := range results {
		// now restart services
		if len(v.RestartServices) > 0 {
			callback := make(chan string)
			for _, unit := range v.RestartServices {
				log.Info("restarting service", "unit", unit)
				_, err := conn.ReloadOrTryRestartUnitContext(ctx, unit, "replace", callback)
				if err != nil {
					log.Warn(err)
				}
				select {
				case res := <-callback:
					log.Debug("restarted unit", "unit", unit, "result", res)
				case <-time.After(time.Duration(timeout) * time.Second):
					log.Warn("timeout while restarting unit", "unit", unit)
				}
			}
		}
		if len(v.NewServices) > 0 {
			callback := make(chan string)
			for _, unit := range v.NewServices {
				log.Info("starting service", "unit", unit)
				_, err := conn.StartUnitContext(ctx, unit, "fail", callback)
				if err != nil {
					log.Warn(err)
				}
				select {
				case res := <-callback:
					log.Debug("started unit", "unit", unit, "result", res)
				case <-time.After(time.Duration(timeout) * time.Second):
					log.Warn("timeout while starting unit", "unit", unit)
				}
			}
		}
	}
	// any remaining cleanup for removed decans?
	for _, v := range results {
		if v.Removed {
			log.Info("removed decan", "decan", v.Decan)
		}
	}
}

type applicationAction struct {
	Name    string
	Enabled bool
}
type applicationResult struct {
	Decan           string
	NeedReload      bool
	RestartServices []string
	NewServices     []string
	Removed         bool
}
