package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	require.NoError(t, exec.Command("materia", "version").Run())
}

func TestCNF(t *testing.T) {
	require.Error(t, exec.Command("materia", "not-found").Run())
}

func TestRepo1_Simple(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", "file:///root/materia/virter/in/testrepo1"))
	require.Nil(t, setEnv("MATERIA_AGE__KEYFILE", "/root/materia/virter/in/testrepo1/test-key.txt"))
	require.Nil(t, setEnv("MATERIA_AGE__BASE_DIR", "secrets"))
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
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "noname"))
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
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", fmt.Sprintf("file://%v", repoPath)))
	require.Nil(t, setEnv("MATERIA_AGE__KEYFILE", fmt.Sprintf("%v/test-key.txt", repoPath)))
	require.Nil(t, setEnv("MATERIA_AGE__BASE_DIR", "secrets"))
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
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "hello")))
	require.True(t, componentInstalled("carpal", filepath.Join(goldenPath, "carpal")))
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "carpal")))
	require.True(t, componentInstalled("double", filepath.Join(goldenPath, "double")))
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "double")))
	require.True(t, componentInstalled("freshrss", filepath.Join(goldenPath, "freshrss")))
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "freshrss")))
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "noname"))
	runCmd = exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentRemoved("hello"))
	require.True(t, servicesStopped(ctx, conn, filepath.Join(goldenPath, "hello")))
	require.True(t, componentRemoved("double"))
	require.True(t, servicesStopped(ctx, conn, filepath.Join(goldenPath, "double")))
	require.True(t, componentRemoved("carpal"))
	require.True(t, servicesStopped(ctx, conn, filepath.Join(goldenPath, "carpal")))
	require.True(t, componentRemoved("freshrss"))
	require.True(t, servicesStopped(ctx, conn, filepath.Join(goldenPath, "freshrss")))
}

func TestRepo3_SOPS(t *testing.T) {
	ctx := context.Background()
	repoPath := "/root/materia/virter/in/testrepo3"
	goldenPath := "/root/materia/virter/out/testrepo3"
	conn, err := connectSystemd(ctx, false)
	require.NoError(t, err, "could't connect to systemd over dbus")
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", fmt.Sprintf("file://%v", repoPath)))
	require.Nil(t, setEnv("MATERIA_SOPS__SUFFIX", "enc"))
	require.Nil(t, setEnv("MATERIA_SOPS__BASE_DIR", "secrets"))
	require.Nil(t, setEnv("SOPS_AGE_KEY_FILE", fmt.Sprintf("%v/test-key.txt", repoPath)))
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
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "hello")))
	require.True(t, componentInstalled("carpal", filepath.Join(goldenPath, "carpal")))
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "carpal")))
	require.True(t, componentInstalled("double", filepath.Join(goldenPath, "double")))
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "double")))
	require.True(t, componentInstalled("freshrss", filepath.Join(goldenPath, "freshrss")))
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "freshrss")))
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "noname"))
	runCmd = exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentRemoved("hello"))
	require.True(t, servicesStopped(ctx, conn, filepath.Join(goldenPath, "hello")))
	require.True(t, componentRemoved("double"))
	require.True(t, servicesStopped(ctx, conn, filepath.Join(goldenPath, "double")))
	require.True(t, componentRemoved("carpal"))
	require.True(t, servicesStopped(ctx, conn, filepath.Join(goldenPath, "carpal")))
	require.True(t, componentRemoved("freshrss"))
	require.True(t, servicesStopped(ctx, conn, filepath.Join(goldenPath, "freshrss")))
}

func TestRepo4_VolumeMigration(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", "file:///root/materia/virter/in/testrepo4"))
	require.Nil(t, setEnv("MATERIA_AGE__KEYFILE", "/root/materia/virter/in/testrepo4/test-key.txt"))
	require.Nil(t, setEnv("MATERIA_AGE__BASE_DIR", "secrets"))
	require.Nil(t, setEnv("MATERIA_MIGRATE_VOLUMES", "true"))
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
	ensureVolumeCmd := exec.Command("systemctl", "start", "hello-volume.service")
	err = ensureVolumeCmd.Run()
	require.NoError(t, err)
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", "file:///root/materia/virter/in/testrepo4_pt2"))
	runCmd = exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
}

func Test_ExampleRepo(t *testing.T) {
	ctx := context.Background()
	conn, err := connectSystemd(ctx, false)
	require.NoError(t, err, "could't connect to systemd over dbus")
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__KIND", "git"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", "https://github.com/stryan/materia_example_repo"))
	require.Nil(t, setEnv("MATERIA_SOPS__SUFFIX", "enc"))
	require.Nil(t, setEnv("MATERIA_SOPS__BASE_DIR", "attributes"))
	require.Nil(t, setEnv("SOPS_AGE_KEY_FILE", "/var/lib/materia/source/key.txt"))
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
	require.True(t, componentExists("freshrss"))
	require.True(t, componentExists("podman_exporter"))
	require.True(t, checkService(ctx, conn, "freshrss.service", "active"))
	require.True(t, checkService(ctx, conn, "podman_exporter.service", "active"))
}

func Test_ExampleRepoBranch(t *testing.T) {
	ctx := context.Background()
	conn, err := connectSystemd(ctx, false)
	require.NoError(t, err, "could't connect to systemd over dbus")
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__KIND", "git"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", "https://github.com/stryan/materia_example_repo"))
	require.Nil(t, setEnv("MATERIA_SOPS__SUFFIX", "enc"))
	require.Nil(t, setEnv("MATERIA_SOPS__BASE_DIR", "attributes"))
	require.Nil(t, setEnv("SOPS_AGE_KEY_FILE", "/var/lib/materia/source/key.txt"))
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
	require.True(t, componentExists("freshrss"))
	require.True(t, componentExists("podman_exporter"))
	require.True(t, checkService(ctx, conn, "freshrss.service", "active"))
	require.True(t, checkService(ctx, conn, "podman_exporter.service", "active"))
	require.Nil(t, setEnv("MATERIA_GIT__BRANCH", "example-branch"))
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd = exec.Command("materia", "update")
	err = runCmd.Run()
	require.NoError(t, err)
	require.False(t, componentExists("freshrss"))
	require.False(t, checkService(ctx, conn, "freshrss.service", "active"))
}

func Test_AllResources(t *testing.T) {
	ctx := context.Background()
	repoPath := "/root/materia/virter/in/testrepo_all"
	goldenPath := "/root/materia/virter/out/testrepo_all"
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", fmt.Sprintf("file://%v", repoPath)))
	require.Nil(t, setEnv("MATERIA_AGE__KEYFILE", fmt.Sprintf("%v/test-key.txt", repoPath)))
	require.Nil(t, setEnv("MATERIA_AGE__BASE_DIR", "secrets"))
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
	require.True(t, componentInstalled("hello", filepath.Join(goldenPath, "hello")))
	require.True(t, scriptExists("hello.sh"), "script not found")
	require.True(t, unitExists("hello.service"))
}

func Test_ContainerWithBuild(t *testing.T) {
	ctx := context.Background()
	conn, err := connectSystemd(ctx, false)
	require.NoError(t, err)
	repoPath := "/root/materia/virter/in/testrepo_build"
	goldenPath := "/root/materia/virter/out/testrepo_build"
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", fmt.Sprintf("file://%v", repoPath)))
	require.Nil(t, setEnv("MATERIA_AGE__KEYFILE", fmt.Sprintf("%v/test-key.txt", repoPath)))
	require.Nil(t, setEnv("MATERIA_AGE__BASE_DIR", "secrets"))
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
	require.True(t, servicesRunning(ctx, conn, filepath.Join(goldenPath, "hello")))
}
