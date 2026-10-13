package facts

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testFacts = &Facts{
	rawFacts: map[string]any{
		"host": map[string]any{
			"hostname": "rem",
			"interfaces": map[string]any{ // at some point lookup what an ipv6 address looks like
				"eth0":      map[string]any{"ip4": []any{"192.168.1.1"}},
				"enp3s0.20": map[string]any{"ip4": []any{"192.168.10.1", "192.168.20.1"}},
			},
		},
		"materia": map[string]any{
			"roles": []any{"support", "dps"},
			"count": json.Number("3"),
		},
	},
	typedFacts: map[string]any{
		"host": HostFacts{Hostname: "rem"},
	},
}

func TestFacts_Lookup(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		path    Path
		input   *Facts
		want    any
		wantErr error
	}{
		{
			name:  "simple",
			path:  Path{"host", "hostname"},
			input: testFacts,
			want:  "rem",
		},
		{
			name:  "networks",
			path:  Path{"host", "interfaces", "eth0"},
			input: testFacts,
			want:  map[string]any{"ip4": []any{"192.168.1.1"}},
		},
		{
			name:  "networks - multiple",
			path:  Path{"host", "interfaces", "enp3s0.20"},
			input: testFacts,
			want:  map[string]any{"ip4": []any{"192.168.10.1", "192.168.20.1"}},
		},
		{
			name:  "networks - index",
			path:  Path{"host", "interfaces", "enp3s0.20", "ip4", "0"},
			input: testFacts,
			want:  "192.168.10.1",
		},
		{
			name:    "not an index",
			input:   testFacts,
			path:    Path{"materia", "roles", "x"},
			wantErr: ErrFactNotFound,
		},
		{
			name:    "index out of range",
			input:   testFacts,
			path:    Path{"materia", "roles", "5"},
			wantErr: ErrFactNotFound,
		},
		{
			name:    "negative index",
			input:   testFacts,
			path:    Path{"materia", "roles", "-1"},
			wantErr: ErrFactNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.Lookup(tt.path)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr, "wanted err %v got %v", tt.wantErr, err)
				return
			}
			require.Equal(t, tt.want, got, "wanted %v got %v", tt.want, got)
		})
	}
}
