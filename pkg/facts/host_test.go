package facts

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_HostCollector(t *testing.T) {
	ctx := context.Background()
	hostname, err := os.Hostname()
	require.Nil(t, err)

	collector := HostCollector{}

	require.Equal(t, "host", collector.Namespace())
	facts, err := Collect(ctx, &collector)
	require.Nil(t, err)

	require.Nil(t, err)
	h, err := ToHostFacts(facts)
	require.NoError(t, err)
	require.Equal(t, hostname, h.Hostname)

	// no way to know what interfaces are on easily so lets just make sure it's not nil
	for name, int := range h.Interfaces {
		assert.NotNil(t, int.Ip4, "interface %s ip4", name)
		assert.NotNil(t, int.Ip6, "interface %s ip6", name)
	}
}

func TestResolveHostname(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)
	tests := []struct {
		name   string
		config string
		want   string
	}{
		{name: "get os name", config: "", want: hostname},
		{name: "get config name", config: "rat_king", want: "rat_king"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveHostname(tt.config)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
