package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/netip"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
)

func startTestContainer(ctx context.Context, bin string) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../../",
			Dockerfile: "Containerfile.test",
			KeepImage:  true,
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.DNS = []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("1.0.0.1")}
			hc.Privileged = true // for podman in podman
		},

		Networks: []string{"podman"},
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("error starting container: %w", err)
	}

	if err := c.CopyFileToContainer(ctx, bin, "/usr/local/bin/materia", 0o755); err != nil {
		_ = c.Terminate(ctx)
		return nil, fmt.Errorf("error installing materia: %w", err)
	}

	return c, nil
}

func runInContainer(ctx context.Context, tc testcontainers.Container, env map[string]string, name string, args ...string) (int, string, error) {
	cmd := append([]string{name}, args...)

	if len(env) > 0 {
		kvs := make([]string, 0, len(env)+len(cmd))
		for k, v := range env {
			kvs = append(kvs, fmt.Sprintf("%s=%s", k, v))
		}
		cmd = append(append([]string{"env"}, kvs...), cmd...)
	}

	code, reader, err := tc.Exec(ctx, cmd)
	if err != nil {
		return code, "", err
	}
	var out, errout bytes.Buffer
	// use Stdcopy to clean up testcontainer output
	if _, err := stdcopy.StdCopy(&out, &errout, reader); err != nil {
		raw, _ := io.ReadAll(reader)
		return code, string(raw), nil
	}
	result := out.String()
	if errout.String() != "" {
		result = fmt.Sprintf("%v\nErr: %v", out.String(), errout.String())
	}
	return code, result, nil
}

func copyDirToContainer(ctx context.Context, c testcontainers.Container, hostDir, containerDir string) error {
	if _, _, err := c.Exec(ctx, []string{"mkdir", "-p", containerDir}); err != nil {
		return err
	}
	return c.CopyDirToContainer(ctx, hostDir, containerDir, 0o644)
}
