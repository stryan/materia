package main

import (
	"context"
	"fmt"
	"strings"

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

func reset(ctx context.Context, tc testcontainers.Container, all bool) error {
	stopTrackedServices(ctx, tc)
	podmanReset := [][]string{
		{"sh", "-c", "podman rm -af"},
		{"sh", "-c", "podman volume prune -f"},
		{"sh", "-c", "podman network prune -f"},
	}
	if all {
		podmanReset = [][]string{
			{"sh", "-c", "podman system reset -f"},
		}
	}
	cmds := [][]string{
		{"rm", "-rf", "/var/lib/materia"},
		{"sh", "-c", "rm -rf /etc/containers/systemd/*"},
		{"sh", "-c", "rm -f /tmp/materia-test-env.sh"},
		{"sh", "-c", "rm -rf /etc/materia/*"},
		{"sh", "-c", "rm -rf /tmp/materia/*"},
	}
	cmds = append(cmds, podmanReset...)
	cmds = append(cmds, [][]string{
		{"sh", "-c", "systemctl reset-failed"},
		{"sh", "-c", "systemctl daemon-reload"},
		{"sh", "-c", "systemctl list-units --state=not-found --no-legend | awk '{print $2}' | xargs -r systemctl stop || true"},
		{"sh", "-c", "systemctl daemon-reload"},
	}...)
	script := buildResetScript(cmds)
	code, out, err := runInContainer(ctx, tc, nil, "sh", "-c", script)
	if err != nil {
		return fmt.Errorf("can't run reset: %w", err)
	}
	if code != 0 {
		if desc, stepCode, ok := parseFailedStep(out); ok {
			return fmt.Errorf("reset failed in container: %v with code %v: %v", desc, stepCode, out)
		}
		return fmt.Errorf("reset failed with code %v: %v", code, out)
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

func buildResetScript(cmds [][]string) string {
	var sb strings.Builder
	sb.WriteString("set -e\n")
	sb.WriteString(`step() {
  __desc="$1"; shift
  if ! "$@"; then
    __code=$?
    echo "__RESET_FAILED__|${__code}|${__desc}" >&2
    exit "$__code"
  fi
}
`)
	for _, c := range cmds {
		desc := strings.Join(c, " ")
		quoted := make([]string, len(c))
		for i, a := range c {
			quoted[i] = shellescape(a)
		}
		fmt.Fprintf(&sb, "step %s %s\n", shellescape(desc), strings.Join(quoted, " "))
	}
	return sb.String()
}

func parseFailedStep(out string) (desc, code string, ok bool) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "__RESET_FAILED__|") {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) == 3 {
			return parts[2], parts[1], true
		}
	}
	return "", "", false
}
