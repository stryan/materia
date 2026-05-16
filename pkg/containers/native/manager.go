package native

import (
	"context"
	"fmt"
	"os"

	"go.podman.io/podman/v6/pkg/bindings"
	"primamateria.systems/materia/pkg/containers"
)

type NativeManager struct {
	secretsPrefix string
	compression   string

	conn context.Context // stores the podman connection
}

func NewNativeManager(ctx context.Context, cfg *containers.ContainersConfig) (*NativeManager, error) {
	p := &NativeManager{
		secretsPrefix: cfg.SecretsPrefix,
		compression:   cfg.Compression,
	}
	socket := "unix:///run/podman/podman.sock"
	if uid := os.Getuid(); uid != 0 {
		socket = fmt.Sprintf("unix:///run/user/%v/podman/podman.sock", uid)
	}
	conn, err := bindings.NewConnection(ctx, socket)
	if err != nil {
		return nil, err
	}
	p.conn = conn
	return p, nil
}

func (p *NativeManager) Close() {
	// in case
}
