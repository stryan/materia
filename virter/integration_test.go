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
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/require"
)

var (
	dataPrefix = "/var/lib/materia/components/"
	quadPrefix = "/etc/containers/systemd/"
)

func clearMateria() error {
	err := os.RemoveAll("/var/lib/materia")
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
	return nil
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
				return fmt.Errorf("Inequal data file %v : %v", dataPath, diffs)
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
				return fmt.Errorf("Inequal quadlet %v : %v", quadPath, diffs)
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

	// Create a new Scanner for the file.
	scanner := bufio.NewScanner(file)

	// Iterate over each line in the file.
	var names []string
	for scanner.Scan() {
		names = append(names, scanner.Text())
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
		}
	}

	return result
}

func clearServices(ctx context.Context, conn *dbus.Conn, goldenPath string) error {
	_, err := os.Stat(goldenPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if errors.Is(err, os.ErrNotExist) {
		return err
	}
	servicesList := filepath.Join(goldenPath, "services")
	if !fileExists(servicesList) {
		return fmt.Errorf("checking services for %v but no services list", goldenPath)
	}

	file, err := os.Open(servicesList)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer func() { _ = file.Close() }()

	// Create a new Scanner for the file.
	scanner := bufio.NewScanner(file)

	// Iterate over each line in the file.
	var names []string
	for scanner.Scan() {
		names = append(names, scanner.Text())
	}
	for _, n := range names {
		err = stopService(ctx, conn, n)
		if err != nil {
			return err
		}
	}
	return nil
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

func TestVersion(t *testing.T) {
	require.NoError(t, exec.Command("materia", "version").Run())
}

func TestCNF(t *testing.T) {
	require.Error(t, exec.Command("materia", "not-found").Run())
}

func TestRepo1_Simple(t *testing.T) {
	require.NoError(t, clearMateria(), "unable to clean up before test")
	require.Nil(t, os.Setenv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, os.Setenv("MATERIA_SOURCE__URL", "file:///root/materia/virter/in/testrepo1"))
	require.Nil(t, os.Setenv("MATERIA_AGE__KEYFILE", "/root/materia/virter/in/testrepo1/test-key.txt"))
	require.Nil(t, os.Setenv("MATERIA_AGE__BASE_DIR", "secrets"))
	planCmd := exec.Command("materia", "plan")
	planCmd.Stdout = os.Stdout
	planCmd.Stderr = os.Stderr
	err := planCmd.Run()
	require.NoError(t, err)
	runCmd := exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentInstalled("hello", "/root/materia/virter/out/testrepo1/hello"))
	require.Nil(t, os.Setenv("MATERIA_HOSTNAME", "noname"))
	runCmd = exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentRemoved("hello"))
}

func TestRepo2_Complex(t *testing.T) {
	ctx := context.Background()
	repoPath := "/root/materia/virter/in/testrepo2"
	goldenPath := "/root/materia/virter/out/testrepo2"
	conn, err := connectSystemd(ctx, false)
	require.NoError(t, err, "could't connect to systemd over dbus")
	require.NoError(t, clearMateria(), "unable to clean up before test")
	require.Nil(t, os.Setenv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, os.Setenv("MATERIA_SOURCE__URL", fmt.Sprintf("file://%v", repoPath)))
	require.Nil(t, os.Setenv("MATERIA_AGE__KEYFILE", fmt.Sprintf("%v/test-key.txt", repoPath)))
	require.Nil(t, os.Setenv("MATERIA_AGE__BASE_DIR", "secrets"))
	planCmd := exec.Command("materia", "plan")
	planCmd.Stdout = os.Stdout
	planCmd.Stderr = os.Stderr
	err = planCmd.Run()
	require.NoError(t, err)
	runCmd := exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentInstalled("hello", filepath.Join(goldenPath, "hello")))
	require.False(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "hello")))
	require.True(t, componentInstalled("carpal", filepath.Join(goldenPath, "carpal")))
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "carpal")))
	require.True(t, componentInstalled("double", filepath.Join(goldenPath, "double")))
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "double")))
	require.True(t, componentInstalled("freshrss", filepath.Join(goldenPath, "freshrss")))
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "freshrss")))
	require.Nil(t, os.Setenv("MATERIA_HOSTNAME", "noname"))
	runCmd = exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentRemoved("hello"))
	require.True(t, componentRemoved("double"))
	require.True(t, componentRemoved("carpal"))
	require.True(t, componentRemoved("freshrss"))
	require.NoError(t, stopService(ctx, conn, "hello.service"))
}

func TestRepo3_SOPS(t *testing.T) {
	ctx := context.Background()
	repoPath := "/root/materia/virter/in/testrepo3"
	goldenPath := "/root/materia/virter/out/testrepo3"
	conn, err := connectSystemd(ctx, false)
	require.NoError(t, err, "could't connect to systemd over dbus")
	require.NoError(t, clearMateria(), "unable to clean up before test")
	require.Nil(t, os.Setenv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, os.Setenv("MATERIA_SOURCE__URL", fmt.Sprintf("file://%v", repoPath)))
	require.Nil(t, os.Setenv("MATERIA_SOPS__SUFFIX", "enc"))
	require.Nil(t, os.Setenv("MATERIA_SOPS__BASE_DIR", "secrets"))
	require.Nil(t, os.Setenv("SOPS_AGE_KEY_FILE", fmt.Sprintf("%v/test-key.txt", repoPath)))
	planCmd := exec.Command("materia", "plan")
	planCmd.Stdout = os.Stdout
	planCmd.Stderr = os.Stderr
	err = planCmd.Run()
	require.NoError(t, err)
	runCmd := exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentInstalled("hello", filepath.Join(goldenPath, "hello")))
	require.False(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "hello")))
	require.True(t, componentInstalled("carpal", filepath.Join(goldenPath, "carpal")))
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "carpal")))
	require.True(t, componentInstalled("double", filepath.Join(goldenPath, "double")))
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "double")))
	require.True(t, componentInstalled("freshrss", filepath.Join(goldenPath, "freshrss")))
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "freshrss")))
	require.Nil(t, os.Setenv("MATERIA_HOSTNAME", "noname"))
	runCmd = exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentRemoved("hello"))
	require.True(t, componentRemoved("double"))
	require.True(t, componentRemoved("carpal"))
	require.True(t, componentRemoved("freshrss"))
	require.NoError(t, stopService(ctx, conn, "hello.service"))
}

func TestRepo4_VolumeMigration(t *testing.T) {
	require.NoError(t, clearMateria(), "unable to clean up before test")
	require.Nil(t, os.Setenv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, os.Setenv("MATERIA_SOURCE__URL", "file:///root/materia/virter/in/testrepo4"))
	require.Nil(t, os.Setenv("MATERIA_AGE__KEYFILE", "/root/materia/virter/in/testrepo4/test-key.txt"))
	require.Nil(t, os.Setenv("MATERIA_AGE__BASE_DIR", "secrets"))
	require.Nil(t, os.Setenv("MATERIA_MIGRATE_VOLUMES", "true"))
	planCmd := exec.Command("materia", "plan")
	planCmd.Stdout = os.Stdout
	planCmd.Stderr = os.Stderr
	err := planCmd.Run()
	require.NoError(t, err)
	runCmd := exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentInstalled("hello", "/root/materia/virter/out/testrepo4/hello"))
	require.Nil(t, os.Setenv("MATERIA_SOURCE__URL", "file:///root/materia/virter/in/testrepo4_pt2"))
	runCmd = exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
}
