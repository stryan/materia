package dirs

import (
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DefaultMateriaDirs(t *testing.T) {
	root := MateriaDirs{
		Data:     "/var/lib/materia",
		Quadlets: "/etc/containers/systemd",
		Services: "/etc/systemd/system",
		Scripts:  "/usr/local/bin",
		Source:   "/var/lib/materia/source",
		Remote:   "/var/lib/materia/remote",
		Output:   "/var/lib/materia/output",
	}
	rootless := MateriaDirs{
		Data:     "/xdg/data/materia",
		Quadlets: "/xdg/config/containers/systemd",
		Services: "/xdg/data/systemd/user",
		Scripts:  "/home/ivy/.local/bin",
		Source:   "/xdg/data/materia/source",
		Remote:   "/xdg/data/materia/remote",
		Output:   "/xdg/data/materia/output",
	}
	normal := &user.User{Uid: "1000", HomeDir: "/home/ivy"}

	tests := []struct {
		name       string
		user       *user.User
		configHome string
		dataHome   string
		want       MateriaDirs
	}{
		{name: "root", user: &user.User{Uid: "0", HomeDir: "/root"}, want: root},
		{name: "nil user", want: root},
		{name: "user", user: normal, configHome: "/xdg/config", dataHome: "/xdg/data", want: rootless},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", tc.configHome)
			t.Setenv("XDG_DATA_HOME", tc.dataHome)

			got, err := DefaultMateriaDirs(tc.user)

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
