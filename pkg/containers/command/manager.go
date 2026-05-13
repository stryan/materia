package command

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"charm.land/log/v2"
	"primamateria.systems/materia/pkg/containers"
)

type CommandManager struct {
	secretsPrefix string
	compression   string
	remote        bool
}

func NewCommandManager(cfg *containers.ContainersConfig) (*CommandManager, error) {
	p := &CommandManager{
		secretsPrefix: cfg.SecretsPrefix,
		remote:        cfg.Remote,
		compression:   cfg.Compression,
	}
	return p, nil
}

func (p *CommandManager) Close() {
}

func genCmd(ctx context.Context, remote bool, args ...string) *exec.Cmd {
	if remote {
		args = append([]string{"--remote"}, args...)
	}
	log.Debugf("podman %v", strings.Join(args, " "))
	return exec.CommandContext(ctx, "podman", args...)
}

func runCmd(cmd *exec.Cmd) (*bytes.Buffer, error) {
	output := bytes.NewBuffer([]byte{})
	errorout := bytes.NewBuffer([]byte{})
	cmd.Stdout = output
	cmd.Stderr = errorout
	err := cmd.Run()
	if err != nil {
		errString := errorout.String()
		if realErr, found := strings.CutPrefix(errString, "Error: "); found {
			return nil, fmt.Errorf("err %w: error from podman command: %v", err, realErr)
		}
	}
	return output, nil
}
