package materia

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"primamateria.systems/materia/internal/testutil"
	"primamateria.systems/materia/pkg/hostman"
	"primamateria.systems/materia/pkg/manifests"
	"primamateria.systems/materia/pkg/mocks"
	"primamateria.systems/materia/pkg/services"
)

func TestNew(t *testing.T) {
	hm := mocks.NewMockHostManager(t)
	sm := mocks.NewMockSourceManager(t)
	sm.EXPECT().LoadManifest(manifests.MateriaManifestFile).Return(&manifests.MateriaManifest{}, nil)
	hm.EXPECT().GetHostname().Return("localhost")
	m, err := NewMateriaFromConfig(context.Background(), &MateriaConfig{
		QuadletDir: "/tmp/materia/quadlets",
		MateriaDir: "/tmp/materia",
		ServiceDir: "/tmp/services",
		ScriptsDir: "/usr/local/bin",
		SourceDir:  "/materia/source",
	}, hm, sm)
	assert.NoError(t, err)
	assert.NotNil(t, m)
}

func TestPlanExecute_InstallsComponent(t *testing.T) {
	ctx := context.Background()
	host := testutil.NewMateriaTestHost(t)

	hmc := &hostman.HostmanConfig{
		Hostname:    "test-host",
		DataDir:     host.DataDir,
		QuadletDir:  host.QuadletDir,
		ScriptsDir:  host.ScriptsDir,
		ServicesDir: host.UnitsDir,
		// Point ServiceManager at the container's dbus socket
		ServicesConfig: &services.ServicesConfig{
			DbusSocket: host.DBusSocket,
		},
	}

	hm, err := hostman.NewHostManager(ctx, hmc)
	require.NoError(t, err)

	m, err := NewMateriaFromConfig(ctx, &MateriaConfig{}, hm, nil)
	require.NoError(t, err)

	plan, err := m.Plan(ctx)
	require.NoError(t, err)

	_, err = m.Execute(ctx, plan)
	require.NoError(t, err)

	// Now assert against real systemd state inside the container
	status := host.Exec(t, "systemctl", "is-active", "my-component.service")
	assert.Equal(t, "active", strings.TrimSpace(status))

	// And the files are visible on the host side too
	_, err = os.Stat(filepath.Join(host.QuadletDir, "my-component.container"))
	assert.NoError(t, err)
}

func containerDBusSocket(t *testing.T, c testcontainers.Container) string {
	t.Helper()
	ctx := context.Background()

	// Get the container's dbus socket path by exec-ing inside it
	// Alternatively expose /run/dbus/system_bus_socket via a bind mount
	// on a host temp path
	exitCode, reader, err := c.Exec(ctx, []string{
		"bash", "-c", "echo $DBUS_SYSTEM_BUS_ADDRESS",
	})
	// ... map it to a host-accessible socket
}
