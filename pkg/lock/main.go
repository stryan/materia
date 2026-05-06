package lock

import (
	"context"
	"errors"
	"fmt"
	"os/user"

	"charm.land/log/v2"
	"github.com/godbus/dbus/v5"
)

const MateriaBusName = "systems.primamateria.materia"

var ErrLockInUse = errors.New("lock in use")

type Locker struct {
	conn   *dbus.Conn
	locked bool
}

func NewLocker(path string) (*Locker, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	isRoot := currentUser.Username == "root"
	l := &Locker{}
	if path == "" {
		if isRoot {
			l.conn, err = dbus.ConnectSystemBus()
			if err != nil {
				return nil, err
			}
		} else {
			l.conn, err = dbus.ConnectSessionBus()
			if err != nil {
				return nil, err
			}
		}
	} else {
		conn, err := dbus.Dial(fmt.Sprintf("unix:path=%s", path))
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
	}

	return l, nil
}

func (l *Locker) Lock() error {
	if l.locked {
		return nil
	}
	reply, err := l.conn.RequestName(MateriaBusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return ErrLockInUse
	}
	l.locked = true
	return nil
}

func (l *Locker) LockOrWait(ctx context.Context) error {
	if l.locked {
		return nil
	}
	matchRule := dbus.WithMatchInterface("org.freedesktop.DBus")
	err := l.conn.AddMatchSignal(
		matchRule,
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, MateriaBusName),
	)
	if err != nil {
		return err
	}
	defer func() {
		err := l.conn.AddMatchSignal(
			matchRule,
			dbus.WithMatchMember("NameOwnerChanged"),
			dbus.WithMatchArg(0, MateriaBusName),
		)
		if err != nil {
			log.Warn("Unable to cleanup materia lock watcher: %v", err)
		}
	}()
	reply, err := l.conn.RequestName(MateriaBusName, 0)
	if err != nil {
		return err
	}
	switch reply {
	case dbus.RequestNameReplyPrimaryOwner:
		return nil
	case dbus.RequestNameReplyInQueue:
		log.Infof("waiting for materia lock: %v", MateriaBusName)
	default:
		return fmt.Errorf("unexpected dbus response when locking: %v", reply)
	}
	signals := make(chan *dbus.Signal, 10)
	l.conn.Signal(signals)
	defer func() {
		l.conn.RemoveSignal(signals)
	}()

	for sig := range signals {
		if sig.Name != "org.freedesktop.DBus.NameAcquired" {
			continue
		}
		name, _ := sig.Body[0].(string)
		if name == MateriaBusName {
			log.Info("aquired materia lock")
			l.locked = true
			return nil
		}
	}
	for {
		select {
		case <-ctx.Done():
			_, err := l.conn.ReleaseName(MateriaBusName) // otherwise dbus will still give us the name
			if err != nil {
				return err
			}
		case sig, ok := <-signals:
			if !ok {
				return errors.New("dbus lock signal closed")
			}
			if sig.Name != "org.freedesktop.DBus.NameAcquired" {
				continue
			}
			name, _ := sig.Body[0].(string)
			if name == MateriaBusName {
				log.Info("aquired materia lock")
				l.locked = true
				return nil
			}

		}
	}
}

func (l *Locker) Unlock() error {
	_, err := l.conn.ReleaseName(MateriaBusName)
	if err != nil {
		return err
	}
	l.locked = false
	return nil
}

func (l *Locker) Close() error {
	// auto releases names
	l.locked = false
	return l.conn.Close()
}
