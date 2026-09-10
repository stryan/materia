package rpc

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_NewVarlinkServer_BindsSocket(t *testing.T) {
	ctx := context.Background()
	addr, err := socketPath()
	require.NoError(t, err)

	serv, err := NewVarlinkServer(ctx, nil, "test")
	require.NoError(t, err)

	require.NoError(t, serv.Bind(ctx, addr))
	defer func() {
		err = serv.Shutdown()
		require.NoError(t, err)
	}()
	path, has := strings.CutPrefix(addr, "unix:")
	require.True(t, has)
	_, err = os.Stat(path)
	require.NoError(t, err)
}
