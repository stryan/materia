package lock

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type FileLock struct {
	f      *os.File
	locked bool
}

func NewFileLock(path string) (*FileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}
	return &FileLock{f: f, locked: false}, nil
}

func (f *FileLock) Lock() error {
	if f.locked {
		return nil
	}
	err := unix.Flock(int(f.f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == unix.EWOULDBLOCK {
		return ErrLockInUse
	}
	if err != nil {
		return fmt.Errorf("flock: %w", err)
	}
	f.locked = true
	return nil
}

func (f *FileLock) LockOrWait(ctx context.Context) error {
	if f.locked {
		return nil
	}
	err := unix.Flock(int(f.f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == nil {
		f.locked = true
		return nil
	}
	if err != unix.EWOULDBLOCK {
		return err
	}
	lockCh := make(chan error, 1)
	go func() {
		// should block here safely
		lockCh <- unix.Flock(int(f.f.Fd()), unix.LOCK_EX)
	}()

	select {
	case err := <-lockCh:
		if err != nil {
			return fmt.Errorf("error getting file lock: %w", err)
		}
		return nil
	case <-ctx.Done():
		return f.f.Close()
	}
}

func (f *FileLock) Unlock() error {
	if !f.locked {
		return nil
	}
	// use flock again so we don't close
	if err := unix.Flock(int(f.f.Fd()), unix.LOCK_UN); err != nil {
		return fmt.Errorf("flock unlock: %w", err)
	}
	f.locked = false
	return nil
}

func (f *FileLock) Close() error {
	return f.f.Close()
}
