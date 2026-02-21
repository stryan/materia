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
	require.True(t, componentInstalled("hello", filepath.Join(goldenPath, "hello")), "hello component not installed")
	require.True(t, scriptExists("hello.sh"), "script not found")
	require.True(t, unitExists("hello_world.service"), "missing hello_world service")
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

func Test_PlannerConfigs(t *testing.T) {
	ctx := context.Background()
	conn, err := connectSystemd(ctx, false)
	require.NoError(t, err)
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", "file:///root/materia/virter/in/testrepo5"))
	require.Nil(t, setEnv("MATERIA_PLANNER__CLEANUP_QUADLETS", "true"))
	require.Nil(t, setEnv("MATERIA_PLANNER__BACKUP_VOLUMES", "false"))
	require.Nil(t, setEnv("MATERIA_AGE__KEYFILE", "/root/materia/virter/in/testrepo5/test-key.txt"))
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
	require.True(t, componentInstalled("hello", "/root/materia/virter/out/testrepo5/hello"))
	require.True(t, servicesRunning(ctx, conn, "/root/materia/virter/out/testrepo5/hello"), "services not running")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "noname"))
	runCmd = exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentRemoved("hello"), "component not removed")
	require.True(t, volumeExists(ctx, "systemd-hello"), "volume should still exist")
	require.True(t, servicesStopped(ctx, conn, "/root/materia/virter/out/testrepo5/hello"), "services still running")
	require.False(t, networkExists(ctx, "systemd-hello"), "network still exists")
}

func Test_EnsureResources(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", "file:///root/materia/virter/in/testrepo5"))
	require.Nil(t, setEnv("MATERIA_AGE__KEYFILE", "/root/materia/virter/in/testrepo5/test-key.txt"))
	require.Nil(t, setEnv("MATERIA_AGE__BASE_DIR", "secrets"))
	runCmd := exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err := runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentInstalled("hello", "/root/materia/virter/out/testrepo5/hello"))
	require.True(t, volumeExists(ctx, "systemd-hello"), "initial volume should exist")
	stopCmd := exec.Command("systemctl", "stop", "hello")
	err = stopCmd.Run()
	require.NoError(t, err)
	removeCmd := exec.Command("podman", "volume", "rm", "systemd-hello")
	err = removeCmd.Run()
	require.NoError(t, err)
	require.False(t, volumeExists(ctx, "systemd-hello"), "volume should not exist")
	runCmd = exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, volumeExists(ctx, "systemd-hello"), "final volume should still exist")
}

func Test_UpdatedResources(t *testing.T) {
	ctx := context.Background()
	conn, err := connectSystemd(ctx, false)
	require.NoError(t, err)
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", "file:///root/materia/virter/in/testrepo6_pt1"))
	require.Nil(t, setEnv("MATERIA_AGE__KEYFILE", "/root/materia/virter/in/testrepo6_pt1/test-key.txt"))
	require.Nil(t, setEnv("MATERIA_AGE__BASE_DIR", "secrets"))
	runCmd := exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentInstalled("hello", "/root/materia/virter/out/testrepo6_pt1/hello"))
	require.True(t, volumeExists(ctx, "systemd-hello"), "volume should exist")
	require.True(t, networkExists(ctx, "systemd-hello"), "network should exist")
	require.True(t, servicesRunning(ctx, conn, "/root/materia/virter/out/testrepo6_pt1/hello"), "services not running")
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", "file:///root/materia/virter/in/testrepo6_pt2"))
	runCmd = exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, servicesRunning(ctx, conn, "/root/materia/virter/out/testrepo6_pt2/hello"), "services not running")
	require.False(t, fileExists("/etc/containers/systemd/hello/hello.volume"))
	require.False(t, fileExists("/etc/containers/systemd/hello/hello.network"))
}

func TestComponentScripts(t *testing.T) {
	ctx := context.Background()
	repoPath := "/root/materia/virter/in/testrepo7"
	goldenPath := "/root/materia/virter/out/testrepo7"
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", fmt.Sprintf("file://%v", repoPath)))
	require.Nil(t, setEnv("MATERIA_AGE__KEYFILE", fmt.Sprintf("%v/test-key.txt", repoPath)))
	require.Nil(t, setEnv("MATERIA_AGE__BASE_DIR", "secrets"))
	runCmd := exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err := runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentInstalled("hello", filepath.Join(goldenPath, "hello")), "hello component not installed")
	require.True(t, fileExists("/tmp/hello"))
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "noname"))
	runCmd = exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentRemoved("hello"))
	require.False(t, fileExists("/tmp/hello"))
}

func TestOCISource(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, clearMateria(ctx))
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__KIND", "oci"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", "oci://git.saintnet.tech/stryan/materia-example-repo:latest"))
	require.Nil(t, setEnv("MATERIA_SOPS__SUFFIX", "enc"))
	require.Nil(t, setEnv("MATERIA_SOPS__BASE_DIR", "attributes"))
	require.Nil(t, setEnv("SOPS_AGE_KEY_FILE", "/var/lib/materia/source/key.txt"))
	planCmd := exec.Command("materia", "plan")
	planCmd.Stdout = os.Stdout
	planCmd.Stderr = os.Stderr
	err := planCmd.Run()
	require.NoError(t, err)
	require.True(t, fileExists("/var/lib/materia/source/components/freshrss/MANIFEST.toml"))
	require.True(t, fileExists("/var/lib/materia/source/components/podman_exporter/MANIFEST.toml"))
}

func TestAllVaults(t *testing.T) {
	ctx := context.Background()
	repoPath := "/root/materia/virter/in/testrepo8"
	goldenPath := "/root/materia/virter/out/testrepo8"
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", fmt.Sprintf("file://%v", repoPath)))
	require.Nil(t, setEnv("MATERIA_SOPS__SUFFIX", "enc"))
	require.Nil(t, setEnv("MATERIA_SOPS__BASE_DIR", "secrets"))
	require.Nil(t, setEnv("MATERIA_SOPS__LOAD_ALL_VAULTS", "true"))
	require.Nil(t, setEnv("SOPS_AGE_KEY_FILE", "/var/lib/materia/source/test-key.txt"))
	runCmd := exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err := runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentInstalled("hello", filepath.Join(goldenPath, "hello")), "hello component not installed")
}

func TestRepo1_AppMode(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, clearMateria(ctx), "unable to clean up before test")
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "localhost"))
	require.Nil(t, setEnv("MATERIA_SOURCE__URL", "file:///root/materia/virter/in/testrepo1"))
	require.Nil(t, setEnv("MATERIA_APPMODE", "true"))
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
	require.True(t, fileExists("/etc/containers/systemd/hello/.hello.app"))
	require.Nil(t, setEnv("MATERIA_HOSTNAME", "noname"))
	runCmd = exec.Command("materia", "update")
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	err = runCmd.Run()
	require.NoError(t, err)
	require.True(t, componentRemoved("hello"))
}
