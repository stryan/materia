package materia

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"primamateria.systems/materia/pkg/repository"
)

func TestMateria_rotateOutput(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		index   int
		max     int
		have    []string
		want    []string
		wantErr bool
	}{
		{
			name:    "empty",
			file:    "test",
			have:    []string{},
			want:    []string{},
			wantErr: false,
		},
		{
			name:    "add",
			file:    "test",
			have:    []string{"test.toml"},
			want:    []string{"test-1.toml"},
			wantErr: false,
		},
		{
			name:    "rotate",
			file:    "test",
			index:   0,
			have:    []string{"test.toml", "test-1.toml", "test-2.toml", "test-3.toml", "test-4.toml"},
			want:    []string{"test-1.toml", "test-2.toml", "test-3.toml", "test-4.toml"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testdir := t.TempDir()
			om, err := repository.NewManagedDir(testdir, 0o755)
			require.Nil(t, err)
			m := Materia{
				Output:    om,
				OutputMax: tt.max,
			}
			if m.OutputMax == 0 {
				m.OutputMax = 5
			}
			for _, v := range tt.have {
				require.Nil(t, om.Install(t.Context(), v, []byte{}))
			}
			gotErr := m.rotateOutput(t.Context(), tt.file, tt.index)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("rotateOutput() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("rotateOutput() succeeded unexpectedly")
			}
			for _, v := range tt.want {
				require.True(t, fileExists(filepath.Join(testdir, v)), "file %v should exist but doesn't", v)
			}
		})
	}
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true // File exists
	}
	if errors.Is(err, os.ErrNotExist) {
		return false // File explicitly does not exist
	}
	// The file may or may not exist (e.g., permission denied, disk error)
	return false
}
