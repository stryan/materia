package testutil

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/docker/docker/api/types/container"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type MateriaTestHost struct {
	connCtx     context.Context
	containerID string
	QuadletDir  string
	DataDir     string
	ScriptsDir  string
	UnitsDir    string
	DBusSocket  string
}

func NewMateriaTestHost(t *testing.T) *MateriaTestHost {
	t.Helper()

	base := t.TempDir()
	h := &MateriaTestHost{
		QuadletDir: filepath.Join(base, "quadlet"),
		DataDir:    filepath.Join(base, "data"),
		ScriptsDir: filepath.Join(base, "scripts"),
		UnitsDir:   filepath.Join(base, "units"),
		DBusSocket: filepath.Join(base, "dbus.sock"),
	}
	for _, d := range []string{h.QuadletDir, h.DataDir, h.ScriptsDir, h.UnitsDir} {
		os.MkdirAll(d, 0o755)
	}
	// must exist as a file before bind-mounting as a socket
	f, err := os.Create(h.DBusSocket)
	if err != nil {
		t.Fatalf("create dbus socket placeholder: %v", err)
	}
	f.Close()

	// Connection is carried in the context, not a client struct
	connCtx, err := bindings.NewConnection(context.Background(), "unix:///run/podman/podman.sock")
	if err != nil {
		t.Fatalf("podman connection: %v", err)
	}
	h.connCtx = connCtx

	s := specgen.NewSpecGenerator("registry.fedoraproject.org/fedora:latest", false)
	s.Command = []string{"/sbin/init"}
	s.Env = map[string]string{"container": "podman"}
	priv := true
	s.Privileged = &priv

	// Bind mounts are first-class in specgen
	s.Mounts = []specs.Mount{
		{Type: "bind", Source: h.QuadletDir, Destination: "/etc/containers/systemd"},
		{Type: "bind", Source: h.DataDir, Destination: "/var/lib/materia"},
		{Type: "bind", Source: h.ScriptsDir, Destination: "/usr/local/lib/materia/scripts"},
		{Type: "bind", Source: h.UnitsDir, Destination: "/etc/systemd/system"},
		{Type: "bind", Source: h.DBusSocket, Destination: "/run/dbus/system_bus_socket"},
	}
	// systemd needs these as tmpfs
	shmSize := int64(65536000) // 64MB
	s.ShmSizeSystemd = &shmSize

	resp, err := containers.CreateWithSpec(connCtx, s, nil)
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	h.containerID = resp.ID

	if err := containers.Start(connCtx, h.containerID, nil); err != nil {
		t.Fatalf("start container: %v", err)
	}

	t.Cleanup(func() {
		containers.Stop(connCtx, h.containerID, nil)
		containers.Remove(connCtx, h.containerID, new(containers.RemoveOptions).WithForce(true))
	})

	h.waitForSystemd(t)
	return h
}

func (h *MateriaTestHost) waitForSystemd(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		rep, err := containers.ExecCreate(h.connCtx, h.containerID, &handlers.ExecCreateConfig{
			ExecOptions: container.ExecOptions{
				Cmd: []string{"systemctl", "is-system-running"},
			},
		})

		// Cmd: []string{"systemctl", "is-system-running"},
		if err == nil {
			if err := containers.ExecStart(h.connCtx, rep, nil); err == nil {
				inspect, err := containers.ExecInspect(h.connCtx, rep, nil)
				if err == nil && inspect.ExitCode == 0 {
					return
				}
			}
		}
		time.Sleep(time.Second)
	}
	t.Fatal("systemd did not become ready within 60s")
}

func (h *MateriaTestHost) Exec(t *testing.T, cmd ...string) (int, bytes.Buffer) {
	t.Helper()
	rep, err := containers.ExecCreate(h.connCtx, h.containerID, &handlers.ExecCreateConfig{
		ExecOptions: container.ExecOptions{
			Cmd:          cmd,
			AttachStdout: true,
			AttachStderr: true,
		},
	})
	if err != nil {
		t.Fatalf("exec create %v: %v", cmd, err)
	}
	var stdout, stderr bytes.Buffer
	err = containers.ExecStartAndAttach(h.connCtx, rep, new(containers.ExecStartAndAttachOptions).
		WithOutputStream(&stdout).
		WithErrorStream(&stderr).
		WithAttachOutput(true).
		WithAttachError(true))
	if err != nil {
		t.Fatalf("exec %v: %v", cmd, err)
	}
	inspect, err := containers.ExecInspect(h.connCtx, rep, nil)
	if err != nil || inspect.ExitCode != 0 {
		t.Fatalf("exec %v exited %d: %s", cmd, inspect.ExitCode, stdout.String())
	}
	return inspect.ExitCode, stdout
}
