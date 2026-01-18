package services

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"time"

	"github.com/charmbracelet/log"
	"github.com/coreos/go-systemd/v22/dbus"
)

var ErrServiceNotFound = errors.New("no service found")

type ServiceManager struct {
	Conn           *dbus.Conn
	isRoot         bool
	DryrunQuadlets bool
}

type Service struct {
	Name    string
	State   string // active, reloading, inactive, failed, activating, deactivating
	Type    string
	Enabled bool
}

func (s Service) Started() bool {
	return s.State == "active"
}

func (s *Service) fillFromProperties(props map[string]interface{}) error {
	jobState := props["ActiveState"].(string)
	fileState := props["UnitFileState"].(string)
	jobType, ok := props["Type"].(string)
	if !ok {
		jobType = "non-existant"
	}
	s.State = jobState
	s.Type = jobType
	s.Enabled = (fileState == "enabled" || fileState == "static")
	return nil
}

//go:generate stringer -type ServiceAction -trimprefix Service
type ServiceAction int

const (
	ServiceStart ServiceAction = iota
	ServiceStop
	ServiceRestart
	ServiceReloadUnits
	ServiceEnable
	ServiceDisable
	ServiceReloadService
)

type ServicesConfig struct {
	Timeout        int
	DryrunQuadlets bool
}

func NewServices(ctx context.Context, cfg *ServicesConfig) (*ServiceManager, error) {
	var sm ServiceManager
	var err error
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}

	if currentUser.Username != "root" {
		sm.Conn, err = dbus.NewUserConnectionContext(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		sm.isRoot = true
		sm.Conn, err = dbus.NewSystemConnectionContext(ctx)
		if err != nil {
			return nil, err
		}

	}
	return &sm, nil
}

func (s *ServiceManager) Apply(ctx context.Context, name string, action ServiceAction, timeout int) error {
	if action == ServiceReloadUnits {
		if s.DryrunQuadlets {
			err := s.dryrunQuadlets(ctx)
			if err != nil {
				return fmt.Errorf("failed quadlet generation while reloading units: %w", err)
			}
		}
		return s.Conn.ReloadContext(ctx)
	}
	callback := make(chan string)
	var err error
	switch action {
	case ServiceRestart:
		_, err = s.Conn.RestartUnitContext(ctx, name, "fail", callback)
	case ServiceEnable:
		_, _, err = s.Conn.EnableUnitFilesContext(ctx, []string{name}, false, false)
		if err != nil {
			return fmt.Errorf("cannot enable unit %v: %w", name, err)
		}
		return nil
	case ServiceDisable:
		_, err = s.Conn.DisableUnitFilesContext(ctx, []string{name}, false)
		if err != nil {
			return fmt.Errorf("cannot disable unit %v: %w", name, err)
		}
		return nil
	case ServiceReloadService:
		_, err = s.Conn.ReloadUnitContext(ctx, name, "fail", callback)
	case ServiceStart:
		_, err = s.Conn.StartUnitContext(ctx, name, "fail", callback)
	case ServiceStop:
		_, err = s.Conn.StopUnitContext(ctx, name, "fail", callback)
	default:
		panic(fmt.Sprintf("unexpected services.ServiceAction: %#v", action))
	}
	if err != nil {
		return fmt.Errorf("error applying service change: %w", err)
	}
	select {
	case <-ctx.Done():
		return errors.New("context cancelled while waiting for service")
	case <-callback:
		return nil
	case <-time.After(time.Duration(timeout) * time.Second):
		return fmt.Errorf("error applying service change for %v: %w", name, errors.New("timeout modifying unit"))
	}
}

func (s *ServiceManager) Get(ctx context.Context, name string) (*Service, error) {
	if name == "" {
		return nil, errors.New("empty service name")
	}
	us, err := s.Conn.ListUnitsByNamesContext(ctx, []string{name})
	if err != nil {
		return nil, fmt.Errorf("couldn't list units: %w", err)
	}
	if len(us) == 0 {
		return nil, ErrServiceNotFound
	}
	props, err := s.Conn.GetAllPropertiesContext(ctx, name)
	if err != nil {
		return nil, err
	}
	result := &Service{Name: name}
	err = result.fillFromProperties(props)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *ServiceManager) WaitUntilState(ctx context.Context, name string, state string, timeout int) error {
	us, err := s.Conn.ListUnitsByNamesContext(ctx, []string{name})
	if err != nil {
		return err
	}
	if len(us) == 0 {
		return ErrServiceNotFound
	}
	props, err := s.Conn.GetAllPropertiesContext(ctx, name)
	if err != nil {
		return err
	}

	activeState := props["ActiveState"]
	if activeState == state {
		return nil
	}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	timeoutTimer := time.NewTimer(time.Duration(timeout) * time.Second)
	defer timeoutTimer.Stop()
	log.Debug("waiting for service to update", "service", name, "state", state, "timeout", timeout)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for service %v to reach state %v", name, state)
		case <-timeoutTimer.C:
			return fmt.Errorf("service %v did not reach state %v", name, state)
		case <-ticker.C:
			props, err := s.Conn.GetAllPropertiesContext(ctx, name)
			if err != nil {
				return err
			}

			activeState := props["ActiveState"]
			if activeState == state {
				return nil
			}
			if activeState == "failed" {
				return fmt.Errorf("service %v in failed state", name)
			}
		}
	}
}

func (s *ServiceManager) RunOneshotCommand(ctx context.Context, timeout int, name string, actions []string) error {
	props := []dbus.Property{
		dbus.PropExecStart(actions, true),
		dbus.PropRemainAfterExit(true),
		dbus.PropType("oneshot"),
	}
	callback := make(chan string)
	_, err := s.Conn.StartTransientUnitContext(ctx, name, "fail", props, callback)
	if err != nil {
		log.Fatal(err)
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("context canceled while waiting for one-shot command %v to finish", actions)
	case e := <-callback:
		if e != "done" {
			return fmt.Errorf("one-shot command %v finished in not done state: %v", actions, e)
		}
	case <-time.After(time.Duration(timeout) * time.Second):
		return fmt.Errorf("timeout while waiting for one-shot command %v to finish", actions)
	}
	return nil
}

func (s *ServiceManager) Close() {
	s.Conn.Close()
}

func (s *ServiceManager) dryrunQuadlets(ctx context.Context) error {
	var cmd *exec.Cmd
	if s.isRoot {
		cmd = exec.CommandContext(ctx, "/usr/lib/systemd/system-generators/podman-system-generator", "--dryrun")
	} else {
		cmd = exec.CommandContext(ctx, "/usr/lib/systemd/system-generators/podman-system-generator", "--user", "--dryrun")
	}
	_, err := cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

type PlannedServiceManager struct {
	ServiceManager
}

func (p *PlannedServiceManager) Get(ctx context.Context, name string) (*Service, error) {
	return nil, ErrServiceNotFound
}
