package services

import (
	"context"
	"errors"
	"fmt"
	"os/user"
	"time"

	"github.com/charmbracelet/log"
	"github.com/coreos/go-systemd/v22/dbus"
)

var ErrServiceNotFound = errors.New("no service found")

type Services interface {
	Apply(context.Context, string, ServiceAction) error
	Get(context.Context, string) (*Service, error)
	WaitUntilState(context.Context, string, string) error
	Close()
}

type ServiceManager struct {
	Conn    *dbus.Conn
	Timeout int
}

type Service struct {
	Name    string
	State   string
	Enabled bool
}

func (s Service) Started() bool {
	return s.State == "active"
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
	Timeout int
}

func NewServices(ctx context.Context, cfg *ServicesConfig) (*ServiceManager, error) {
	var sm ServiceManager
	var err error
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	if cfg.Timeout == 0 {
		sm.Timeout = 60
	} else {
		sm.Timeout = cfg.Timeout
	}

	if currentUser.Username != "root" {
		sm.Conn, err = dbus.NewUserConnectionContext(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		sm.Conn, err = dbus.NewSystemConnectionContext(ctx)
		if err != nil {
			return nil, err
		}

	}
	return &sm, nil
}

func (s *ServiceManager) Apply(ctx context.Context, name string, action ServiceAction) error {
	if action == ServiceReloadUnits {
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
			return err
		}
		return nil
	case ServiceDisable:
		_, err = s.Conn.DisableUnitFilesContext(ctx, []string{name}, false)
		if err != nil {
			return err
		}
		return nil
	case ServiceReloadService:
		_, err = s.Conn.ReloadUnitContext(ctx, name, "", callback)
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
	case <-callback:
		return nil
	case <-time.After(time.Duration(s.Timeout) * time.Second):
		return fmt.Errorf("error applying service change for %v: %w", name, errors.New("timeout modifying unit"))
	}
}

func (s *ServiceManager) Get(ctx context.Context, name string) (*Service, error) {
	us, err := s.Conn.ListUnitsByNamesContext(ctx, []string{name})
	if err != nil {
		return nil, err
	}
	if len(us) == 0 {
		return nil, fmt.Errorf("error getting service %v: %w", name, ErrServiceNotFound)
	}
	if len(us) != 1 {
		return nil, errors.New("too many units returned")
	}
	file, err := s.Conn.ListUnitFilesByPatternsContext(ctx, []string{"enabled"}, []string{name})
	if err != nil {
		return nil, err
	}
	return &Service{
		Name:    us[0].Name,
		State:   us[0].ActiveState,
		Enabled: len(file) > 0,
	}, nil
}

func (s *ServiceManager) WaitUntilState(ctx context.Context, name string, state string) error {
	ticker := time.NewTicker(1 * time.Second)
	us, err := s.Conn.ListUnitsByNamesContext(ctx, []string{name})
	if err != nil {
		return err
	}
	if len(us) == 0 {
		return ErrServiceNotFound
	}
	serv := us[0]
	if serv.ActiveState == state {
		return nil
	}
	log.Debugf("waiting on %v to reach %v", name, state)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for service %v to reach state %v", name, state)
		case <-time.After(time.Duration(s.Timeout) * time.Second):
			return fmt.Errorf("service %v did not reach state %v", name, state)
		case <-ticker.C:
			us, err := s.Conn.ListUnitsByNamesContext(ctx, []string{name})
			if err != nil {
				return err
			}
			if len(us) == 0 {
				return ErrServiceNotFound
			}
			serv := us[0]
			if serv.ActiveState == state {
				return nil
			}

			if serv.ActiveState == state {
				return nil
			}
			if serv.ActiveState == "failed" {
				return fmt.Errorf("service %v in failed state", name)
			}
		}
	}
}

func (s *ServiceManager) Close() {
	s.Conn.Close()
}

type PlannedServiceManager struct {
	ServiceManager
}

func (p *PlannedServiceManager) Get(ctx context.Context, name string) (*Service, error) {
	return nil, ErrServiceNotFound
}
