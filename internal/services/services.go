package services

import (
	"context"
	"errors"
	"fmt"
	"os/user"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

type Services interface {
	Apply(context.Context, string, ServiceAction) error
	Get(context.Context, string) (*Service, error)
	Close()
}

type ServiceManager struct {
	Conn    *dbus.Conn
	Timeout int
}

type Service struct {
	Name  string
	State string
}

//go:generate stringer -type ServiceAction -trimprefix Service
type ServiceAction int

const (
	ServiceStart ServiceAction = iota
	ServiceStop
	ServiceRestart
	ServiceReload
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
	if action == ServiceReload {
		return s.Conn.ReloadContext(ctx)
	}
	callback := make(chan string)
	var err error
	switch action {
	case ServiceRestart:
		_, err = s.Conn.RestartUnitContext(ctx, name, "fail", callback)
	case ServiceStart:
		if strings.HasSuffix(name, ".timer") {
			_, _, err = s.Conn.EnableUnitFilesContext(ctx, []string{name}, false, false)
			if err != nil {
				return err
			}
		}
		_, err = s.Conn.StartUnitContext(ctx, name, "fail", callback)
	case ServiceStop:
		if strings.HasSuffix(name, ".timer") {
			_, err = s.Conn.DisableUnitFilesContext(ctx, []string{name}, false)
			if err != nil {
				return err
			}
		}
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
		return fmt.Errorf("error applying service change: %w", errors.New("timeout restarting unit"))
	}
}

func (s *ServiceManager) Get(ctx context.Context, name string) (*Service, error) {
	us, err := s.Conn.ListUnitsByNamesContext(ctx, []string{name})
	if err != nil {
		return nil, err
	}
	if len(us) != 1 {
		return nil, errors.New("too many units returned")
	}
	return &Service{
		Name:  us[0].Name,
		State: us[0].ActiveState,
	}, nil
}

func (s *ServiceManager) Close() {
	s.Conn.Close()
}
