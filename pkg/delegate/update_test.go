package delegate

import (
	"context"
	"errors"
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

func serve(t *testing.T, address string, impl *fakeServer) {
	t.Helper()
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
	address := "unix:" + filepath.Join(socketDir(t), "materia.sock")
	serve(t, address, &fakeServer{})
	steps, err := updateSocket(context.Background(), address, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, int64(7), steps)
}

func TestUpdateSocket_ReportsFailure(t *testing.T) {
	address := "unix:" + filepath.Join(socketDir(t), "materia.sock")
	serve(t, address, &fakeServer{fail: true})
	steps, err := updateSocket(context.Background(), address, 5*time.Second)
	require.Error(t, err)
	assert.Equal(t, int64(2), steps)
	assert.ErrorContains(t, err, "service unhealthy")
	assert.ErrorContains(t, err, "2 of 5")
}

func TestUpdateSocket_WaitsForServer(t *testing.T) {
	address := "unix:" + filepath.Join(socketDir(t), "materia.sock")
	go func() {
		time.Sleep(700 * time.Millisecond)
		serve(t, address, &fakeServer{})
	}()
	steps, err := updateSocket(context.Background(), address, 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, int64(7), steps)
}

func TestUpdateSocket_GivesUp(t *testing.T) {
	address := "unix:" + filepath.Join(socketDir(t), "materia.sock")
	start := time.Now()
	_, err := updateSocket(context.Background(), address, 0)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestUpdateAccount_UnknownAccount(t *testing.T) {
	_, err := UpdateAccount(context.Background(), "materia-no-such-account", 0)
	assert.True(t, errors.Is(err, ErrNoAccount), "got %v", err)
}

func TestSocketPath(t *testing.T) {
	assert.Equal(t, "unix:/run/user/975/materia/materia.sock", SocketPath("975"))
}
