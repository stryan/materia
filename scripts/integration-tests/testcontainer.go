package main

import (
	"context"
	"fmt"
	"io"

	"github.com/testcontainers/testcontainers-go"
)

func startTestContainer(ctx context.Context, bin string) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../../",
			Dockerfile: "Containerfile.test",
			KeepImage:  true,
		},
		// needs privliedged for podman in podman
		Privileged: true,
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
	out, _ := io.ReadAll(reader)
	return code, string(out), nil
}

func copyDirToContainer(ctx context.Context, c testcontainers.Container, hostDir, containerDir string) error {
	if _, _, err := c.Exec(ctx, []string{"mkdir", "-p", containerDir}); err != nil {
		return err
	}
	return c.CopyDirToContainer(ctx, hostDir, containerDir, 0o644)
}
