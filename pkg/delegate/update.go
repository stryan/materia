package delegate

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os/user"
	"path/filepath"
	"syscall"
	"time"

	varlinkapi "primamateria.systems/materia/pkg/api"
)

var (
	// ErrNoAccount means the account of a delegated component does not exist.
	ErrNoAccount = errors.New("account does not exist")
	// ErrReplyTooLarge means the account's materia sent more than a reply to Update can hold.
	ErrReplyTooLarge = errors.New("reply is too large")
)

const (
	// connectRetry is how often UpdateAccount tries again while the account's
	// materia is not listening yet.
	connectRetry = 500 * time.Millisecond
	// maxReply is the largest reply, in bytes, that UpdateAccount reads. A
	// reply to Update holds one number, or an error text and two numbers.
	maxReply = 64 * 1024

	updateMethod = "systems.primamateria.materia.Update"
	updateFailed = "systems.primamateria.materia.UpdateFailed"
)

// SocketPath is the varlink socket of the materia server that runs as the
// user with the given numeric ID. It is the server's default socket.
func SocketPath(uid string) string {
	return filepath.Join("/run/user", uid, "materia", "materia.sock")
}

// UpdateAccount asks the materia server of account to sync its source, plan
// and execute. It returns the number of steps that server completed.
//
// The account's user manager may still be starting when this runs, so
// UpdateAccount keeps trying to connect for up to wait.
//
// The account owns the socket, so UpdateAccount treats whatever answers
// there as untrusted: it speaks the one call itself and reads a bounded
// reply.
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

func updateSocket(ctx context.Context, path string, wait time.Duration) (int64, error) {
	conn, err := connect(ctx, path, wait)
	if err != nil {
		return 0, fmt.Errorf("can't reach materia server at %v: %w", path, err)
	}
	defer func() { _ = conn.Close() }()
	// closing the connection is what interrupts a read when ctx ends
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	steps, err := callUpdate(conn)
	if err != nil && ctx.Err() != nil {
		return steps, ctx.Err()
	}
	return steps, err
}

// callUpdate sends one varlink Update call on conn and reads its reply.
func callUpdate(conn io.ReadWriter) (int64, error) {
	call, err := json.Marshal(struct {
		Method string `json:"method"`
	}{updateMethod})
	if err != nil {
		return 0, err
	}
	if _, err := conn.Write(append(call, 0)); err != nil {
		return 0, err
	}
	raw, err := bufio.NewReader(io.LimitReader(conn, maxReply+1)).ReadBytes(0)
	switch {
	case len(raw) > maxReply:
		return 0, ErrReplyTooLarge
	case errors.Is(err, io.EOF):
		return 0, io.ErrUnexpectedEOF
	case err != nil:
		return 0, err
	}
	var reply struct {
		Parameters json.RawMessage `json:"parameters"`
		Error      string          `json:"error"`
	}
	if err := json.Unmarshal(raw[:len(raw)-1], &reply); err != nil {
		return 0, fmt.Errorf("invalid reply: %w", err)
	}
	switch reply.Error {
	case "":
		var out struct {
			Steps int64 `json:"steps"`
		}
		if err := unmarshalParameters(reply.Parameters, &out); err != nil {
			return 0, err
		}
		return out.Steps, nil
	case updateFailed:
		var failed varlinkapi.UpdateFailed
		if err := unmarshalParameters(reply.Parameters, &failed); err != nil {
			return 0, err
		}
		return failed.StepsCompleted, fmt.Errorf("update failed after %v of %v steps: %v", failed.StepsCompleted, failed.TotalSteps, failed.Message)
	default:
		return 0, fmt.Errorf("update failed: %v", reply.Error)
	}
}

func unmarshalParameters(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("invalid reply: %w", err)
	}
	return nil
}

func connect(ctx context.Context, path string, wait time.Duration) (*net.UnixConn, error) {
	deadline := time.Now().Add(wait)
	var dialer net.Dialer
	for {
		conn, err := dialer.DialContext(ctx, "unix", path)
		if err == nil {
			return conn.(*net.UnixConn), nil
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
