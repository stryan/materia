package materia

import "context"

type Locker interface {
	Lock() error
	LockOrWait(context.Context) error
	Unlock() error
	Close() error
}
