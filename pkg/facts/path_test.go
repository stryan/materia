package facts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePath(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		key     string
		want    Path
		wantErr bool
	}{
		{
			name: "simple",
			key:  "host",
			want: Path{"host"},
		},
		{
			name:    "empty",
			key:     "",
			wantErr: true,
		},
		{
			name: "dotted",
			key:  "host.name.is",
			want: Path{"host", "name", "is"},
		},

		{
			name: "quoted",
			key:  "host.interface.\"eth.0\"",
			want: Path{"host", "interface", "eth.0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := ParsePath(tt.key)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("ParsePath() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("ParsePath() succeeded unexpectedly")
			}
			require.Equal(t, tt.want, got, "got %v wanted %v", got, tt.want)
		})
	}
}
