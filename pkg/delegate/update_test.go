package delegate

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
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
}

func (s *fakeServer) Update(ctx context.Context, c varlinkapi.VarlinkCall) error {
	if s.fail {
		return c.ReplyUpdateFailed(ctx, "service unhealthy", 2, 5)
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
	steps, err := updateSocket(context.Background(), path, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, int64(7), steps)
}

func TestUpdateSocket_ReportsFailure(t *testing.T) {
	path := filepath.Join(socketDir(t), "materia.sock")
	serve(t, path, &fakeServer{fail: true})
	steps, err := updateSocket(context.Background(), path, 5*time.Second)
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
	steps, err := updateSocket(context.Background(), path, 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, int64(7), steps)
}

func TestUpdateSocket_GivesUp(t *testing.T) {
	path := filepath.Join(socketDir(t), "materia.sock")
	start := time.Now()
	_, err := updateSocket(context.Background(), path, 0)
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
	_, err := updateSocket(context.Background(), path, time.Second)
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
			_, err := updateSocket(context.Background(), path, time.Second)
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
	_, err := updateSocket(ctx, path, time.Second)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 3*time.Second)
}
