package services

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/coreos/go-systemd/v22/dbus"
	godbus "github.com/godbus/dbus/v5"
)

var (
	ErrServiceNotFound   = errors.New("no service found")
	ErrOperationTimedOut = errors.New("operation timeout")
	ErrStateChangeFailed = errors.New("service state change failed")
)

type ServiceManager struct {
	Conn           *dbus.Conn
	isRoot         bool
	DryrunQuadlets bool
}

type Service struct {
	Name    string
	State   ServiceState
	Type    string
	Enabled ServiceEnableState
}

func (s Service) Started() bool {
	return s.State == StateActive
}

func (s *Service) fillFromProperties(props map[string]interface{}) error {
	jobState, ok := props["ActiveState"].(string)
	if !ok {
		return fmt.Errorf("invalid active state: %v", props["ActiveState"])
	}
	fileState, ok := props["UnitFileState"].(string)
	if !ok {
		return fmt.Errorf("invalid unit file state state: %v", props["UnitFileState"])
	}
	jobType, ok := props["Type"].(string)
	if !ok {
		jobType = "non-existent"
	}
	s.State = NewServiceState(jobState)
	s.Type = jobType
	s.Enabled = NewServiceEnableState(fileState)
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

func NewServices(ctx context.Context, cfg *ServicesConfig) (*ServiceManager, error) {
	var sm ServiceManager
	var err error
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	sm.DryrunQuadlets = cfg.DryrunQuadlets
	sm.isRoot = currentUser.Username == "root"

	if cfg.DbusSocket == "" {
		if sm.isRoot {
			sm.Conn, err = dbus.NewSystemConnectionContext(ctx)
			if err != nil {
				return nil, err
			}
		} else {
			sm.Conn, err = dbus.NewUserConnectionContext(ctx)
			if err != nil {
				return nil, err
			}

		}
	} else {
		sm.Conn, err = NewSystemdConnection(cfg.DbusSocket)
		if err != nil {
			return nil, err
		}
	}
	return &sm, nil
}

func (s *ServiceManager) ApplyService(ctx context.Context, name string, action ServiceAction, timeout int) error {
	if action == ServiceReloadUnits {
		if s.DryrunQuadlets {
			err := s.dryrunQuadlets(ctx)
			if err != nil {
				return fmt.Errorf("failed quadlet generation while reloading units: %w", err)
			}
		}
		return s.Conn.ReloadContext(ctx)
	}
	if timeout == 0 {
		timeout = 30
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
		return fmt.Errorf("unexpected services.ServiceAction: %#v", action)
	}
	if err != nil {
		return fmt.Errorf("error applying service change: %w", err)
	}
	err = waitForCallback(ctx, callback, timeout)
	if err != nil {
		return fmt.Errorf("error applying service change for %v: %w", name, err)
	}
	return nil
}

func (s *ServiceManager) GetService(ctx context.Context, name string) (*Service, error) {
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
	// FIXME use something more robust
	if us[0].LoadState == "not-found" {
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

func (s *ServiceManager) WaitUntilState(ctx context.Context, name string, state ServiceState, timeout int) error {
	if state == StateInternalWildcard {
		// Nothing to do, we allow all states
		return nil
	}
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
	count := 0
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for service %v to reach state %v", name, state)
		case <-timeoutTimer.C:
			return fmt.Errorf("%w: service %v did not reach state %v", ErrOperationTimedOut, name, state)
		case <-ticker.C:
			props, err := s.Conn.GetAllPropertiesContext(ctx, name)
			if err != nil {
				return err
			}

			rawState := props["ActiveState"]
			activeState := NewServiceState(rawState.(string))
			if activeState == state {
				return nil
			}
			if activeState == "failed" {
				return ErrStateChangeFailed
			}
		}
		count++
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
		return err
	}

	err = waitForCallback(ctx, callback, timeout)
	if err != nil {
		return fmt.Errorf("error running oneshot operation: %w", err)
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
	res, err := cmd.Output()
	if err != nil {
		log.Debug(res)
		return err
	}
	return nil
}

func waitForCallback(ctx context.Context, callback chan string, timeout int) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during systemd operation")
	case result := <-callback:
		if result != "done" {
			if result == "failed" {
				return ErrStateChangeFailed
			}
			return fmt.Errorf("finished with status: %s", result)
		}
		return nil
	case <-time.After(time.Duration(timeout) * time.Second):
		return ErrOperationTimedOut
	}
}

func NewSystemdConnection(socketPath string) (*dbus.Conn, error) {
	return dbus.NewConnection(func() (*godbus.Conn, error) {
		conn, err := godbus.Dial(fmt.Sprintf("unix:path=%s", socketPath))
		if err != nil {
			return nil, err
		}
		if err := conn.Auth(nil); err != nil {
			_ = conn.Close()
			return nil, err
		}
		if err := conn.Hello(); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return conn, nil
	})
}

func PathToService(name string) string {
	kind := filepath.Ext(name)
	switch kind {
	case ".container":
		return strings.ReplaceAll(name, ".container", ".service")
	case ".kube":
		return strings.ReplaceAll(name, ".kube", ".service")
	case ".pod":
		return strings.ReplaceAll(name, ".pod", "-pod.service")
	case ".network":
		return strings.ReplaceAll(name, ".network", "-network.service")
	case ".volume":
		return strings.ReplaceAll(name, ".volume", "-volume.service")
	case ".build":
		return strings.ReplaceAll(name, ".build", "-build.service")
	case ".image":
		return strings.ReplaceAll(name, ".image", "-image.service")
	default:
		return name
	}
}
