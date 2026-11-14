package delegate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/user"
	"path/filepath"
	"syscall"
	"time"

	"github.com/varlink/go/varlink"
	varlinkapi "primamateria.systems/materia/pkg/api"
)

// ErrNoAccount means the account of a delegated component does not exist.
var ErrNoAccount = errors.New("account does not exist")

// connectRetry is how often UpdateAccount tries again while the account's
// materia is not listening yet.
const connectRetry = 500 * time.Millisecond

// SocketPath is the varlink address of the materia server that runs as the
// user with the given numeric ID. It is the server's default socket.
func SocketPath(uid string) string {
	return fmt.Sprintf("unix:%v", filepath.Join("/run/user", uid, "materia", "materia.sock"))
}

// UpdateAccount asks the materia server of account to sync its source, plan
// and execute. It returns the number of steps that server completed.
//
// The account's user manager may still be starting when this runs, so
// UpdateAccount keeps trying to connect for up to wait.
func UpdateAccount(ctx context.Context, account string, wait time.Duration) (int64, error) {
	u, err := user.Lookup(account)
	if err != nil {
		if _, ok := errors.AsType[user.UnknownUserError](err); ok {
			return 0, fmt.Errorf("%w: %v", ErrNoAccount, account)
		}
		return 0, fmt.Errorf("can't look up account %v: %w", account, err)
	}
	return updateSocket(ctx, SocketPath(u.Uid), wait)
}

func updateSocket(ctx context.Context, address string, wait time.Duration) (int64, error) {
	conn, err := connect(ctx, address, wait)
	if err != nil {
		return 0, fmt.Errorf("can't reach materia server at %v: %w", address, err)
	}
	defer func() { _ = conn.Close() }()
	steps, err := varlinkapi.Update().Call(ctx, conn)
	if err != nil {
		if failed, ok := errors.AsType[*varlinkapi.UpdateFailed](varlinkapi.Dispatch_Error(err)); ok {
			return failed.StepsCompleted, fmt.Errorf("update failed after %v of %v steps: %v", failed.StepsCompleted, failed.TotalSteps, failed.Message)
		}
		return 0, err
	}
	return steps, nil
}

func connect(ctx context.Context, address string, wait time.Duration) (*varlink.Connection, error) {
	deadline := time.Now().Add(wait)
	for {
		conn, err := varlink.NewConnection(ctx, address)
		if err == nil {
			return conn, nil
		}
		notListening := errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED)
		if !notListening || !time.Now().Before(deadline) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(connectRetry):
		}
	}
}
