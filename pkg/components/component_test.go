package components

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewComponent(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Component
		wantErr bool
	}{
		{
			name:  "happy",
			input: "web",
			want: &Component{
				Name:     "web",
				Instance: "",
			},
		},
		{
			name:  "happy - instanced",
			input: "web@foo",
			want: &Component{
				Name:     "web",
				Instance: "foo",
			},
		},
		{
			name:    "sad - too many @",
			input:   "web@foo@bar",
			wantErr: true,
		},
		{
			name:    "sad - ..",
			input:   "../foo",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := NewComponent(tt.input)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("NewComponent() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("NewComponent() succeeded unexpectedly")
			}
			require.Equal(t, tt.want.Name, got.Name)
			require.Equal(t, tt.want.Instance, got.Instance)
		})
	}
}

func TestIsDropinDir(t *testing.T) {
	tests := []struct {
		name string
		file string
		want bool
	}{
		{
			name: "happy1",
			file: "hello.container.d",
			want: true,
		},
		{
			name: "happy2",
			file: "hello@.container.d",
			want: true,
		},
		{
			name: "happy3",
			file: "hello-.container.d",
			want: true,
		},
		{
			name: "sad",
			file: "hello.container",
		},
		{
			name: "sad - nested",
			file: "conf/hello.container.d",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDropinDir(tt.file)
			require.Equal(t, tt.want, got)
		})
	}
}
