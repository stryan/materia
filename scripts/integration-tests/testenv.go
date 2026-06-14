package main

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
)

var (
	materiaEnv      []string
	materiaServices []string
)

func clearEnv(ctx context.Context, tc testcontainers.Container) {
	for _, k := range materiaEnv {
		_, _, _ = tc.Exec(ctx, []string{
			"sh", "-c",
			fmt.Sprintf("sed -i '/^export %s=/d' /tmp/materia-test-env.sh", k),
		})
	}
	materiaEnv = nil
}

func setEnv(ctx context.Context, tc testcontainers.Container, key, value string) error {
	line := fmt.Sprintf("export %s=%q\n", key, value)
	_, _, err := tc.Exec(ctx, []string{
		"sh", "-c",
		fmt.Sprintf("echo %s >> /tmp/materia-test-env.sh", shellescape(line)),
	})
	if err != nil {
		return err
	}
	materiaEnv = append(materiaEnv, key)
	return nil
}

func reset(ctx context.Context, tc testcontainers.Container) error {
	stopTrackedServices(ctx, tc)
	cmds := [][]string{
		{"rm", "-rf", "/var/lib/materia"},
		{"sh", "-c", "rm -rf /etc/containers/systemd/*"},
		{"sh", "-c", "rm -f /tmp/materia-test-env.sh"},
		{"sh", "-c", "rm -rf /etc/materia/*"},
		{"sh", "-c", "rm -rf /tmp/materia/*"},
		{"sh", "-c", "podman system reset -f"},
		{"sh", "-c", "systemctl reset-failed"},
		{"sh", "-c", "systemctl daemon-reload"},
		{"sh", "-c", "systemctl list-units --state=not-found --no-legend | awk '{print $2}' | xargs -r systemctl stop || true"},
		{"sh", "-c", "systemctl daemon-reload"},
	}
	for _, c := range cmds {
		if code, out, err := runInContainer(ctx, tc, nil, c[0], c[1:]...); err != nil || code != 0 {
			return fmt.Errorf("reset failed on %v with code %v: %v %w", c, code, out, err)
		}
	}
	clearEnv(ctx, tc)

	return nil
}

func trackServices(tc TestCase) {
	materiaServices = append(materiaServices, tc.Output.ActiveServices...)
}

func stopTrackedServices(ctx context.Context, tc testcontainers.Container) {
	for _, s := range materiaServices {
		_ = applyService(ctx, tc, s, "stop")
	}
	materiaServices = nil
}
