package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	"github.com/charmbracelet/log"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sergi/go-diff/diffmatchpatch"
)

var (
	dataPrefix = "/var/lib/materia/components/"
	quadPrefix = "/etc/containers/systemd/"
)

var (
	envVars         = []string{}
	runningServices = []string{}
)

func clearMateria(ctx context.Context) error {
	conn, err := connectSystemd(ctx, false)
	if err != nil {
		return err
	}

	err = os.RemoveAll("/var/lib/materia")
	if err != nil {
		return err
	}
	entries, err := os.ReadDir("/etc/containers/systemd")
	if err != nil {
		return err
	}
	for _, e := range entries {
		err = os.RemoveAll(filepath.Join("/etc/containers/systemd", e.Name()))
		if err != nil {
			return err
		}
	}
	for _, v := range envVars {
		err = os.Unsetenv(v)
		if err != nil {
			return err
		}
	}
	envVars = []string{}
	for _, s := range runningServices {
		err := stopService(ctx, conn, s)
		if err != nil {
			return err
		}
	}
	scriptedservs, err := conn.ListUnitsByPatternsContext(ctx, []string{"active"}, []string{"*-materia-setup.service", "*-materia-cleanup.service"})
	if err != nil {
		return err
	}
	for _, ss := range scriptedservs {
		err := stopService(ctx, conn, ss.Name)
		if err != nil {
			return err
		}
	}
	runningServices = []string{}
	return nil
}

func setEnv(key, value string) error {
	envVars = append(envVars, key)
	return os.Setenv(key, value)
}

func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	log.Fatalf("Error checking file existence for %s: %v\n", filePath, err)
	return false
}

func fileEqual(src, dest string) ([]diffmatchpatch.Diff, error) {
	var diffs []diffmatchpatch.Diff
	dmp := diffmatchpatch.New()
	srcBytes, err := os.ReadFile(src)
	if err != nil {
		return diffs, errors.New("can't read source file")
	}
	srcContent := string(srcBytes)
	destBytes, err := os.ReadFile(dest)
	if err != nil {
		return diffs, errors.New("can't read dest file")
	}
	destContent := string(destBytes)
	return dmp.DiffMain(srcContent, destContent, false), nil
}

func componentExists(name string) bool {
	_, err := os.Stat(filepath.Join("/var/lib/materia/components", name))
	return err == nil
}

func scriptExists(name string) bool {
	return fileExists(filepath.Join("/usr/local/bin/", name))
}

func unitExists(name string) bool {
	return fileExists(filepath.Join("/etc/systemd/system", name))
}

func volumeExists(ctx context.Context, name string) bool {
	cmd := exec.CommandContext(ctx, "podman", "volume", "exists", name)
	err := cmd.Run()
	return err == nil
}

func networkExists(ctx context.Context, name string) bool {
	cmd := exec.CommandContext(ctx, "podman", "network", "exists", name)
	err := cmd.Run()
	return err == nil
}

func componentInstalled(name, goldenPath string) bool {
	err := filepath.WalkDir(filepath.Join(goldenPath, "data"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dataPath, err := filepath.Rel(filepath.Join(goldenPath, "data"), path)
		if err != nil {
			return err
		}
		dfile := filepath.Join(dataPrefix, name, dataPath)
		if !fileExists(dfile) {
			return fmt.Errorf("no such data file: %v", dataPath)
		}
		if !d.IsDir() {
			diffs, err := fileEqual(dfile, path)
			if err != nil {
				return fmt.Errorf("error comparing data files: %w", err)
			}
			if len(diffs) > 1 {
				return fmt.Errorf("inequal data file %v : %v", dataPath, diffs)
			}
		}
		return nil
	})
	if err != nil {
		log.Warn(err)
		return false
	}
	err = filepath.WalkDir(filepath.Join(goldenPath, "quadlets"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		quadPath, err := filepath.Rel(filepath.Join(goldenPath, "quadlets"), path)
		if err != nil {
			return err
		}
		qfile := filepath.Join(quadPrefix, name, quadPath)
		if !fileExists(qfile) {
			return fmt.Errorf("no such quadlet: %v", quadPath)
		}
		if !d.IsDir() {
			diffs, err := fileEqual(qfile, path)
			if err != nil {
				return fmt.Errorf("error comparing quadlets: %w", err)
			}
			if len(diffs) > 1 {
				return fmt.Errorf("inequal quadlet %v : %v", quadPath, diffs)
			}
		}
		return nil
	})
	if err != nil {
		log.Warn(err)
		return false
	}
	return true
}

func componentRemoved(name string) bool {
	return !fileExists(filepath.Join(dataPrefix, name)) && !fileExists(filepath.Join(quadPrefix, name))
}

func connectSystemd(ctx context.Context, user bool) (*dbus.Conn, error) {
	var conn *dbus.Conn
	var err error
	if user {
		conn, err = dbus.NewUserConnectionContext(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		conn, err = dbus.NewSystemConnectionContext(ctx)
		if err != nil {
			return nil, err
		}

	}
	return conn, nil
}

func checkService(ctx context.Context, conn *dbus.Conn, name string, state string) bool {
	states, err := conn.ListUnitsByNamesContext(ctx, []string{name})
	if err != nil {
		log.Fatal(err)
	}
	if len(states) == 0 {
		return false
	}
	result := true
	for _, s := range states {
		if s.ActiveState != state {
			log.Warnf("service %v isn't running", s.Name)
			result = false
		}
	}
	return result
}

func servicesRunning(ctx context.Context, conn *dbus.Conn, goldenPath string) bool {
	_, err := os.Stat(goldenPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("component not found in golden path: %v", goldenPath)
	}
	servicesList := filepath.Join(goldenPath, "services")
	if !fileExists(servicesList) {
		log.Warnf("checking services for %v but no services list", goldenPath)
		return false
	}

	file, err := os.Open(servicesList)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)

	var names []string
	for scanner.Scan() {
		names = append(names, scanner.Text())
	}
	if len(names) == 0 {
		return true
	}
	states, err := conn.ListUnitsByNamesContext(ctx, names)
	if err != nil {
		log.Fatal(err)
	}
	if len(states) == 0 {
		return false
	}
	result := true
	for _, s := range states {
		if s.ActiveState != "active" {
			log.Warnf("service %v isn't running", s.Name)
			result = false
		} else {
			runningServices = append(runningServices, s.Name)
		}
	}

	return result
}

func servicesStopped(ctx context.Context, conn *dbus.Conn, goldenPath string) bool {
	_, err := os.Stat(goldenPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("component not found in golden path: %v", goldenPath)
	}
	servicesList := filepath.Join(goldenPath, "services")
	if !fileExists(servicesList) {
		log.Warnf("checking services for %v but no services list", goldenPath)
		return false
	}

	file, err := os.Open(servicesList)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)

	var names []string
	for scanner.Scan() {
		names = append(names, scanner.Text())
	}
	if len(names) == 0 {
		return true
	}
	states, err := conn.ListUnitsByNamesContext(ctx, names)
	if err != nil {
		log.Fatal(err)
	}
	if len(states) == 0 {
		return false
	}
	result := true
	for _, s := range states {
		if s.ActiveState != "inactive" {
			log.Warnf("service %v is running", s.Name)
			result = false
		} else {
			for k, v := range runningServices {
				if v == s.Name {
					runningServices = slices.Delete(runningServices, k, k+1)
					break
				}
			}
		}
	}
	return result
}

func stopService(ctx context.Context, conn *dbus.Conn, n string) error {
	callback := make(chan string)
	_, err := conn.StopUnitContext(ctx, n, "fail", callback)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("context canceled while waiting to stop unit %v", n)
	case <-callback:
	case <-time.After(time.Duration(30) * time.Second):
		return fmt.Errorf("error stopping unit %v: %w", n, errors.New("timeout stopping unit"))
	}
	return nil
}
