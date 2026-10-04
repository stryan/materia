package components

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResource_GetMode(t *testing.T) {
	tests := []struct {
		name  string
		input Resource
		want  os.FileMode
	}{
		{
			name: "directory",
			input: Resource{
				Path: "conf",
				Mode: 0,
				Kind: ResourceTypeDirectory,
			},
			want: 0o755,
		},
		{
			name: "quadlet",
			input: Resource{
				Path: "hello.container",
				Mode: 0o755,
				Kind: ResourceTypeContainer,
			},
			want: 0o644,
		},
		{
			name: "script",
			input: Resource{
				Path: "hello.sh",
				Mode: 0o600,
				Kind: ResourceTypeScript,
			},
			want: 0o755,
		},
		{
			name: "file1",
			input: Resource{
				Mode: 0o700,
				Kind: ResourceTypeFile,
			},
			want: 0o755,
		},
		{
			name: "file2",
			input: Resource{
				Mode: 0o600,
				Kind: ResourceTypeFile,
			},
			want: 0o644,
		},
		{
			name: "file3",
			input: Resource{
				Mode: 0o600,
				Kind: ResourceTypeFile,
			},
			want: 0o644,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.GetMode()
			require.Equal(t, tt.want, got)
		})
	}
}
