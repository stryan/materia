package materia

import (
	"context"
	"errors"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

type Services interface {
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Restart(context.Context, string) error
	Reload(context.Context) error
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

func (s *ServiceManager) Start(ctx context.Context, name string) error {
	callback := make(chan string)
	_, err := s.Conn.StartUnitContext(ctx, name, "fail", callback)
	if err != nil {
		return err
	}
	select {
	case <-callback:
		return nil
	case <-time.After(time.Duration(s.Timeout) * time.Second):
		return errors.New("timeout starting unit")
	}
}

func (s *ServiceManager) Stop(ctx context.Context, name string) error {
	callback := make(chan string)
	_, err := s.Conn.StopUnitContext(ctx, name, "fail", callback)
	if err != nil {
		return err
	}
	select {
	case <-callback:
		return nil
	case <-time.After(time.Duration(s.Timeout) * time.Second):
		return errors.New("timeout stopping unit")
	}
}

func (s *ServiceManager) Restart(ctx context.Context, name string) error {
	callback := make(chan string)
	_, err := s.Conn.RestartUnitContext(ctx, name, "fail", callback)
	if err != nil {
		return err
	}
	select {
	case <-callback:
		return nil
	case <-time.After(time.Duration(s.Timeout) * time.Second):
		return errors.New("timeout restarting unit")
	}
}

func (s *ServiceManager) Reload(ctx context.Context) error {
	callback := make(chan string)
	err := s.Conn.ReloadContext(ctx)
	if err != nil {
		return err
	}
	select {
	case <-callback:
		return nil
	case <-time.After(time.Duration(s.Timeout) * time.Second):
		return errors.New("timeout starting unit")
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
