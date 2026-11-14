package delegate

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/varlink/go/varlink"
	varlinkapi "primamateria.systems/materia/pkg/api"
)

type fakeServer struct {
	varlinkapi.VarlinkInterface
	fail bool
	// message replaces the text of the failure
	message string
}

func (s *fakeServer) Update(ctx context.Context, c varlinkapi.VarlinkCall) error {
	if s.fail {
		message := "service unhealthy"
		if s.message != "" {
			message = s.message
		}
		return c.ReplyUpdateFailed(ctx, message, 2, 5)
	}
	return c.ReplyUpdate(ctx, 7)
}

// socketDir returns a directory short enough for a unix socket path.
func socketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "mt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func serve(t *testing.T, path string, impl *fakeServer) {
	t.Helper()
	address := "unix:" + path
	service, err := varlink.NewService("test", "materia", "0", "https://example.invalid")
	require.NoError(t, err)
	require.NoError(t, service.RegisterInterface(varlinkapi.VarlinkNew(impl)))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = service.Listen(ctx, address, 0)
	}()
	t.Cleanup(func() {
		cancel()
		_ = service.Shutdown()
		<-done
	})
}

func TestUpdateSocket(t *testing.T) {
	path := filepath.Join(socketDir(t), "materia.sock")
	serve(t, path, &fakeServer{})
	steps, err := updateSocket(context.Background(), path, me(), 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, int64(7), steps)
}

func TestUpdateSocket_ReportsFailure(t *testing.T) {
	path := filepath.Join(socketDir(t), "materia.sock")
	serve(t, path, &fakeServer{fail: true})
	steps, err := updateSocket(context.Background(), path, me(), 5*time.Second)
	require.Error(t, err)
	assert.Equal(t, int64(2), steps)
	assert.ErrorContains(t, err, "service unhealthy")
	assert.ErrorContains(t, err, "2 of 5")
}

func TestUpdateSocket_WaitsForServer(t *testing.T) {
	path := filepath.Join(socketDir(t), "materia.sock")
	go func() {
		time.Sleep(700 * time.Millisecond)
		serve(t, path, &fakeServer{})
	}()
	steps, err := updateSocket(context.Background(), path, me(), 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, int64(7), steps)
}

func TestUpdateSocket_GivesUp(t *testing.T) {
	path := filepath.Join(socketDir(t), "materia.sock")
	start := time.Now()
	_, err := updateSocket(context.Background(), path, me(), 0)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestUpdateAccount_UnknownAccount(t *testing.T) {
	_, err := UpdateAccount(context.Background(), "materia-no-such-account", 0)
	assert.True(t, errors.Is(err, ErrNoAccount), "got %v", err)
}

func TestSocketPath(t *testing.T) {
	assert.Equal(t, "/run/user/975/materia/materia.sock", SocketPath("975"))
}

// serveRaw answers every connection on path with handle, without varlink.
func serveRaw(t *testing.T, path string, handle func(net.Conn)) {
	t.Helper()
	listener, err := net.Listen("unix", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				handle(conn)
			}()
		}
	}()
}

func TestUpdateSocket_RejectsEndlessReply(t *testing.T) {
	path := filepath.Join(socketDir(t), "materia.sock")
	serveRaw(t, path, func(conn net.Conn) {
		chunk := bytes.Repeat([]byte("a"), 4096)
		for {
			if _, err := conn.Write(chunk); err != nil {
				return
			}
		}
	})
	_, err := updateSocket(context.Background(), path, me(), time.Second)
	assert.ErrorIs(t, err, ErrReplyTooLarge)
}

func TestUpdateSocket_RejectsInvalidReply(t *testing.T) {
	for name, reply := range map[string]string{
		"not json":       "hello\x00",
		"wrong type":     "{\"parameters\":{\"steps\":\"many\"}}\x00",
		"closed early":   "{\"parameters\":",
		"unknown error":  "{\"error\":\"org.varlink.service.MethodNotFound\"}\x00",
		"nothing at all": "",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(socketDir(t), "materia.sock")
			serveRaw(t, path, func(conn net.Conn) { _, _ = conn.Write([]byte(reply)) })
			_, err := updateSocket(context.Background(), path, me(), time.Second)
			assert.Error(t, err)
		})
	}
}

func TestUpdateSocket_StopsWithContext(t *testing.T) {
	path := filepath.Join(socketDir(t), "materia.sock")
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	serveRaw(t, path, func(net.Conn) { <-hold })
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := updateSocket(ctx, path, me(), time.Second)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 3*time.Second)
}

// me is the user ID the test servers run as.
func me() uint32 { return uint32(os.Getuid()) }

func TestUpdateSocket_RejectsWrongPeer(t *testing.T) {
	path := filepath.Join(socketDir(t), "materia.sock")
	var called atomic.Bool
	closed := make(chan struct{})
	serveRaw(t, path, func(conn net.Conn) {
		defer close(closed)
		buffer := make([]byte, 1)
		if n, _ := conn.Read(buffer); n > 0 {
			called.Store(true)
		}
	})
	_, err := updateSocket(context.Background(), path, me()+1, time.Second)
	assert.ErrorIs(t, err, ErrWrongPeer)
	<-closed
	assert.False(t, called.Load(), "nothing may be sent to a peer that is not the account")
}

func TestUpdateSocket_RejectsRedirect(t *testing.T) {
	dir := socketDir(t)
	other := filepath.Join(dir, "other.sock")
	serve(t, other, &fakeServer{})
	link := filepath.Join(dir, "materia.sock")
	require.NoError(t, os.Symlink(other, link))
	// the link reaches a server that runs as someone other than the account
	_, err := updateSocket(context.Background(), link, me()+1, time.Second)
	assert.ErrorIs(t, err, ErrWrongPeer)
}

func TestUpdateSocket_CleansFailureText(t *testing.T) {
	path := filepath.Join(socketDir(t), "materia.sock")
	hostile := "bad\nERRO forged line\x1b[2J\r" + strings.Repeat("x", 2*maxMessage)
	serve(t, path, &fakeServer{fail: true, message: hostile})
	_, err := updateSocket(context.Background(), path, me(), 5*time.Second)
	require.Error(t, err)
	text := err.Error()
	assert.NotContains(t, text, "\n")
	assert.NotContains(t, text, "\r")
	assert.NotContains(t, text, "\x1b")
	assert.Contains(t, text, "forged line", "the text itself is kept, on one line")
	assert.Less(t, len(text), maxMessage*4+200)
	assert.True(t, strings.HasSuffix(text, "…"))
}

func TestPrintable(t *testing.T) {
	assert.Equal(t, "plain text, with ünïcödé", printable("plain text, with ünïcödé"))
	assert.Equal(t, "a\ufffdb\ufffdc", printable("a\x00b\tc"))
	assert.Equal(t, strings.Repeat("y", maxMessage), printable(strings.Repeat("y", maxMessage)))
}
