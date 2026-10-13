package facts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_normalize(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		v       any
		want    map[string]any
		wantErr bool
	}{
		{
			name: "host facts",
			v: HostFacts{
				Hostname: "ivy",
			},
			want: map[string]any{
				"hostname":   "ivy",
				"interfaces": nil,
			},
			wantErr: false,
		},
		{name: "need an object", v: []string{"wrong-move-idiot"}, wantErr: true},
		{name: "unmarshalable value", v: make(chan int), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := normalize(tt.v)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("normalize() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("normalize() succeeded unexpectedly")
			}
			require.Equal(t, tt.want, got, "wanted: %v got %v", tt.want, got)
		})
	}
}
