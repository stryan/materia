package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_RemoteRegistry(t *testing.T) {
	fix := newSrcFixture(t)
	reg := fix.reg
	assert.Equal(t, 0, reg.Size())

	_ = fix.remote(t, "web", "")
	require.NoError(t, reg.Register("web", ""))
	_ = fix.remote(t, "foo", "bar")
	require.NoError(t, reg.Register("foo", "bar"))

	assert.Equal(t, 2, reg.Size())

	reg.Reset()
	assert.Equal(t, 0, reg.Size())
}

func Test_RemoteComponentRegistry_EmptyRoot(t *testing.T) {
	reg, err := NewRemoteComponentRegistry("")
	require.NoError(t, err)
	require.ErrorIs(t, reg.Register("any", ""), ErrNoRemoteDir)
}
