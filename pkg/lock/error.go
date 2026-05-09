package lock

import "errors"

var ErrLockInUse = errors.New("lock in use")
