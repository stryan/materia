package containers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/log"
)

var ErrPodmanObjectNotFound error = errors.New("no such object")

type PodmanManager struct {
	secretsPrefix string
	remote        bool
}

func NewPodmanManager(remote bool, prefix string) (*PodmanManager, error) {
	p := &PodmanManager{
		secretsPrefix: prefix,
		remote:        remote,
	}
	return p, nil
}

func (p *PodmanManager) Close() {
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
