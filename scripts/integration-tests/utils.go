package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/testcontainers/testcontainers-go"
)

func runMateriaCmd(ctx context.Context, tc testcontainers.Container, args ...string) error {
	escapedArgs := make([]string, len(args))
	for i, a := range args {
		escapedArgs[i] = shellescape(a)
	}
	cmd := fmt.Sprintf("materia %v", strings.Join(escapedArgs, " "))
	fullCmd := fmt.Sprintf("set -e; [ -f /tmp/materia-test-env.sh ] && . /tmp/materia-test-env.sh; %s", cmd)

	code, output, err := runInContainer(ctx, tc, nil, "sh", "-c", fullCmd)
	log.Info(output)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("error running materia %v: ec %v", strings.Join(args, " "), code)
	}
	return nil
}

func fileExists(ctx context.Context, tc testcontainers.Container, path string) bool {
	code, _, _ := runInContainer(ctx, tc, nil, "test", "-e", path)
	return code == 0
}

func getFile(ctx context.Context, c testcontainers.Container, path string) ([]byte, error) {
	reader, err := c.CopyFileFromContainer(ctx, path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	return io.ReadAll(reader)
}

func writeFile(ctx context.Context, tc testcontainers.Container, path, content string) error {
	file, err := os.CreateTemp("", "materia-test-file-")
	if err != nil {
		return err
	}

	_, err = file.Write([]byte(content))
	if err != nil {
		fErr := file.Close()
		return errors.Join(err, fErr)
	}
	err = file.Close()
	if err != nil {
		return err
	}
	return tc.CopyFileToContainer(ctx, file.Name(), path, 0o755)
}

func compareFile(ctx context.Context, tc testcontainers.Container, left, right string) ([]diffmatchpatch.Diff, error) {
	dmp := diffmatchpatch.New()

	leftOut, err := getFile(ctx, tc, left)
	if err != nil {
		return nil, fmt.Errorf("can't read %s: %w", left, err)
	}
	rightOut, err := getFile(ctx, tc, right)
	if err != nil {
		return nil, fmt.Errorf("can't read %s: %w", right, err)
	}
	return dmp.DiffMain(string(leftOut), string(rightOut), false), nil
}

func volumeExists(ctx context.Context, tc testcontainers.Container, name string) bool {
	code, _, _ := runInContainer(ctx, tc, nil, "podman", "volume", "exists", name)
	return code == 0
}

func networkExists(ctx context.Context, tc testcontainers.Container, name string) bool {
	code, _, _ := runInContainer(ctx, tc, nil, "podman", "network", "exists", name)
	return code == 0
}

func getService(ctx context.Context, tc testcontainers.Container, name, state string) bool {
	_, out, _ := runInContainer(ctx, tc, nil, "systemctl", "show", "--property=ActiveState", name)
	line := strings.TrimSpace(out)
	return line == "ActiveState="+state
}

func reloadServices(ctx context.Context, tc testcontainers.Container) error {
	_, _, err := runInContainer(ctx, tc, nil, "systemctl", "daemon-reload")
	if err != nil {
		return err
	}
	return nil
}

func applyService(ctx context.Context, tc testcontainers.Container, name, action string) error {
	timeout := time.Now().Add(30 * time.Second)
	// TODO this does not work the way I want it to
	for {
		code, _, err := runInContainer(ctx, tc, nil, "systemctl", action, name)
		if err != nil {
			return err
		}
		if code == 0 {
			return nil
		}
		if time.Now().After(timeout) {
			return fmt.Errorf("timeout applying action %v to %v", action, name)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func queryContainer(ctx context.Context, tc testcontainers.Container, containerName, format string) (string, error) {
	code, out, err := runInContainer(ctx, tc, nil, "podman", "inspect", "--format", format, containerName)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("podman inspect exited %d for container %q: %s", code, containerName, out)
	}

	return strings.TrimSpace(out), nil
}

func listInstalledComponents(ctx context.Context, c testcontainers.Container) ([]string, error) {
	_, reader, err := c.Exec(ctx, []string{"find", "/var/lib/materia/components", "-mindepth", "1", "-maxdepth", "1", "-type", "d"})
	if err != nil {
		return nil, fmt.Errorf("exec find: %w", err)
	}

	// strip the annoying testcontainer headers
	var buf bytes.Buffer
	if _, err := stdcopy.StdCopy(&buf, io.Discard, reader); err != nil {
		return nil, fmt.Errorf("reading output: %w", err)
	}

	var dirs []string
	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			dirs = append(dirs, filepath.Base(line))
		}
	}
	return dirs, scanner.Err()
}

func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// SOPS doesn't have a stable encryption package so we're just gonna shim it
func sopsEncryptFile(ctx context.Context, pubkey, src string) error {
	var outfile *os.File
	var err error
	filename := filepath.Base(src)
	extension := filepath.Ext(filename)
	parent := filepath.Dir(src)
	basename := strings.TrimSuffix(filename, extension)

	dest := fmt.Sprintf("%v/%v.enc.yml", parent, basename)
	outfile, err = os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = outfile.Close() }()
	cmd := exec.CommandContext(ctx,
		"sops", "encrypt",
		"--age", pubkey,
		src,
	)
	errbuf := bytes.NewBuffer([]byte{})
	cmd.Stdout = outfile
	cmd.Stderr = errbuf
	if err := cmd.Run(); err != nil {
		fmt.Printf("sops err: %v", errbuf)
	}
	return nil
}
