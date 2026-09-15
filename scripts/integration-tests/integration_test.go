package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

var tc testcontainers.Container

func TestMain(m *testing.M) {
	ctx := context.Background()

	var err error
	tc, err = startTestContainer(ctx, "../../bin/materia-amd64")
	if err != nil {
		log.Fatalf("failed to start test container: %v\n", err)
	}
	if tc == nil {
		log.Fatal("no test container")
	}
	for i := range testcases {
		if err := testcases[i].Setup(); err != nil {
			log.Fatalf("failed to set up test case %v: %v\n", testcases[i].Name, err)
		}
	}
	if err := installTestCase(ctx, tc, testcases...); err != nil {
		log.Fatalf("failed to bulk install fixtures: %v\n", err)
	}

	ec := m.Run()
	if keep := os.Getenv("MATERIA_KEEP_TEST_CONTAINER"); keep == "true" {
		os.Exit(ec)
	}
	err = tc.Terminate(ctx)
	if err != nil {
		log.Fatalf("error terminating test container: %v", err)
	}
	os.Exit(ec)
}

func TestVersion(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, runMateriaCmd(ctx, tc, "version"))
}

func TestCNF(t *testing.T) {
	ctx := context.Background()
	require.Error(t, runMateriaCmd(ctx, tc, "not-found"))
}

func TestRepo1_SimpleCase(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := simpleRepo
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "plan"))
	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
	testcase.Output = TestOutput{}
	require.NoError(t, setEnv(ctx, tc, "MATERIA_HOSTNAME", "noname"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func TestRepo2_ComplexCase(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := simpleRepo2
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "plan"))
	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
}

func Test_Sops(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := sopsTest
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, setEnv(ctx, tc, "SOPS_AGE_KEY_FILE", filepath.Join(testcase.Destination(), "config", "key.txt")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_VolumeMigration(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase1 := migration1
	testcase2 := migration2
	trackServices(testcase2)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase1.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
	require.NoError(t, checkTestCase(ctx, tc, testcase1))

	// double check volume is created
	code, _, err := runInContainer(ctx, tc, nil, "systemctl", "start", "hello-volume.service")
	require.NoError(t, err)
	require.Zero(t, code)

	// create test file
	code, _, err = runInContainer(ctx, tc, nil, "bash", "-c", "touch /var/lib/containers/storage/volumes/systemd-hello/_data/testfile")
	require.NoError(t, err)
	require.Zero(t, code)

	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase2.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
	require.NoError(t, checkTestCase(ctx, tc, testcase2))
	// TODO check that the volume has a new label too
	require.True(t, fileExists(ctx, tc, "/var/lib/containers/storage/volumes/systemd-hello/_data/testfile"))
}

func Test_ExampleRepo(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := exampleRepo
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, setEnv(ctx, tc, "SOPS_AGE_KEY_FILE", "/var/lib/materia/source/key.txt"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_ExampleRepoBranch(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := exampleRepoBranch
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, setEnv(ctx, tc, "SOPS_AGE_KEY_FILE", "/var/lib/materia/source/key.txt"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))

	require.NoError(t, setEnv(ctx, tc, "MATERIA_GIT__BRANCH", "example-branch"))
	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	testcase.Output.InactiveServices = []string{"freshrss.service"}
	testcase.Output.ActiveServices = []string{"podman_exporter.service"}
	testcase.Output.Components = []string{"podman_exporter"}
	testcase.Output.Files = exampleRepoPodmanExporterOutput

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_Rollback_Git_Failed(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	code, result, err := runInContainer(ctx, tc, nil, "git", "clone", "https://github.com/stryan/materia_example_repo", "/tmp/materia/repo")
	require.NoError(t, err, "unable to run clone command")
	require.Zero(t, code, "failed to clone repo: %w", result)
	testcase := rollbackGitFailed
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, setEnv(ctx, tc, "SOPS_AGE_KEY_FILE", "/var/lib/materia/source/key.txt"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))

	// now break the repo with a new commit
	badManifest := `
	Defaults.containerTag = "brokenNotReal"
	Defaults.Port = 9882

	[[Services]]
	Service = "podman_exporter.service"
	RestartedBy = ["podman_exporter.container"]
	`
	require.NoError(t, writeFile(ctx, tc, "/tmp/materia/repo/components/podman_exporter/MANIFEST.toml", badManifest))
	code, result, err = runInContainer(ctx, tc, nil, "git", "-C", "/tmp/materia/repo", "add", "components/podman_exporter/MANIFEST.toml")
	require.NoError(t, err, "unable to run git add")
	require.Zero(t, code, "failed to edit repo: %w", result)

	code, result, err = runInContainer(ctx, tc, nil, "git", "-C", "/tmp/materia/repo", "commit", "-m", "\"broken commit\"")
	require.NoError(t, err, "unable to run git commit")
	require.Zero(t, code, "failed to edit repo: %w", result)

	require.Error(t, runMateriaCmd(ctx, tc, "update"))
	testcase.Output.ActiveServices = []string{"freshrss.service"}
	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_Rollback_Git_Success(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	code, result, err := runInContainer(ctx, tc, nil, "git", "clone", "https://github.com/stryan/materia_example_repo", "/tmp/materia/repo")
	require.NoError(t, err, "unable to run clone command")
	require.Zero(t, code, "failed to clone repo: %w", result)
	testcase := rollbackGitSuccess
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, setEnv(ctx, tc, "SOPS_AGE_KEY_FILE", "/var/lib/materia/source/key.txt"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))

	// now break the repo with a new commit
	badManifest := `
	Defaults.containerTag = "brokenNotReal"
	Defaults.Port = 9882

	[[Services]]
	Service = "podman_exporter.service"
	RestartedBy = ["podman_exporter.container"]
	`
	require.NoError(t, writeFile(ctx, tc, "/tmp/materia/repo/components/podman_exporter/MANIFEST.toml", badManifest))
	code, result, err = runInContainer(ctx, tc, nil, "git", "-C", "/tmp/materia/repo", "add", "components/podman_exporter/MANIFEST.toml")
	require.NoError(t, err, "unable to run git add")
	require.Zero(t, code, "failed to edit repo: %w", result)

	code, result, err = runInContainer(ctx, tc, nil, "git", "-C", "/tmp/materia/repo", "commit", "-m", "\"broken commit\"")
	require.NoError(t, err, "unable to run git commit")
	require.Zero(t, code, "failed to edit repo: %w", result)

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
}

func Test_AllResources(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := allResources
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_ContainerWithBuild(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := containerWithBuild
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_PlannerConfigs(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := plannerConfigs
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))

	testcase.Output = TestOutput{}
	require.NoError(t, setEnv(ctx, tc, "MATERIA_HOSTNAME", "noname"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
	require.True(t, volumeExists(ctx, tc, "systemd-hello"), "volume should survive")
	require.False(t, networkExists(ctx, tc, "systemd-hello"), "network should be removed")
}

func Test_EnsureQuadlets(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := ensureQuadlets
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))

	// Stop container service and remove volume resource
	err := applyService(ctx, tc, "hello.service", "stop")
	require.NoError(t, err)
	code, result, err := runInContainer(ctx, tc, nil, "podman", "volume", "rm", "systemd-hello")
	require.NoError(t, err)
	require.Zero(t, code, "failed to remove volume: %w", result)
	require.False(t, volumeExists(ctx, tc, "systemd-hello"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
	require.True(t, volumeExists(ctx, tc, "systemd-hello"), "volume should be recreated")
	require.NoError(t, reset(ctx, tc, false))
}

func Test_UpdatedResources(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase1 := updatedRes1
	testcase2 := updatedRes2
	trackServices(testcase2)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase1.Destination(), "config", "config.toml")))
	require.NoError(t, setEnv(ctx, tc, "MATERIA_DEBUG", "1"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
	require.NoError(t, checkTestCase(ctx, tc, testcase1))

	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase2.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
	require.NoError(t, checkTestCase(ctx, tc, testcase2))
}

func Test_ComponentScripts(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := componentScripts
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
	require.True(t, fileExists(ctx, tc, "/tmp/hello"))
	testcase.Output = TestOutput{}
	require.NoError(t, setEnv(ctx, tc, "MATERIA_HOSTNAME", "noname"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
	require.NoError(t, checkTestCase(ctx, tc, testcase))
	require.False(t, fileExists(ctx, tc, "/tmp/hello"))
}

func Test_OCISource(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := ociSource
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, setEnv(ctx, tc, "SOPS_AGE_KEY_FILE", "/var/lib/materia/source/key.txt"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_OCISource_RollbackFailed(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := rollbackOciFailed
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, setEnv(ctx, tc, "SOPS_AGE_KEY_FILE", "/var/lib/materia/source/key.txt"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	// Now swap to a broken repo image
	require.NoError(t, setEnv(ctx, tc, "MATERIA_SOURCE__URL", "oci://git.saintnet.tech/stryan/materia-example-repo:bad"))
	testcase.Output.ActiveServices = []string{"freshrss.service"}

	require.Error(t, runMateriaCmd(ctx, tc, "update"))
	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_OCISource_RollbackSuccess(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := rollbackOciSuccess
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, setEnv(ctx, tc, "SOPS_AGE_KEY_FILE", "/var/lib/materia/source/key.txt"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	// Now swap to a broken repo image
	require.NoError(t, setEnv(ctx, tc, "MATERIA_SOURCE__URL", "oci://git.saintnet.tech/stryan/materia-example-repo:bad"))
	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_AppMode(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := appMode
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_QuadletDropins(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := quadletDropins
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, writeFile(ctx, tc, "/etc/containers/systemd/hello/hello.container.d/override.conf", "[Container]\nImage=docker.io/busybox:stable\n"))
	require.NoError(t, reloadServices(ctx, tc))
	require.NoError(t, applyService(ctx, tc, "hello", "restart"))

	info, err := queryContainer(ctx, tc, "busybox1", "{{ .ImageName }}")
	require.Nil(t, err, "couldn't get container info ", err)
	require.Equal(t, "docker.io/library/busybox:stable", info, "image not equal: ", info)

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	// second reload+restart to make sure if it *did* remove the override, we see it
	require.NoError(t, reloadServices(ctx, tc))
	require.NoError(t, applyService(ctx, tc, "hello", "restart"))

	info, err = queryContainer(ctx, tc, "busybox1", "{{ .ImageName }}")
	require.Nil(t, err, "couldn't get container info ", err)
	require.Equal(t, "docker.io/library/busybox:stable", info, "image not equal: ", info)

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_InstancedComponents(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := instancedComponents
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_ServerMode(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := serverMode
	trackServices(testcase)

	require.NoError(t, runMateriaServer(ctx, tc, testcase), "materia server failed to start")
	require.NoError(t, waitForFile(ctx, tc, "/run/materia/materia.sock", true, 30*time.Second))

	require.NoError(t, checkTestCase(ctx, tc, testcase))

	require.NoError(t, stopMateriaServer(ctx, tc, testcase), "unable to stop server")
	require.NoError(t, waitForFile(ctx, tc, "/run/materia/materia.sock", false, 30*time.Second))
}

func Test_ServerMode_AutoPlan(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := serverModePlan
	trackServices(testcase)

	require.NoError(t, runMateriaServer(ctx, tc, testcase), "materia server failed to start")
	require.NoError(t, waitForFile(ctx, tc, "/run/materia/materia.sock", true, 30*time.Second))

	require.NoError(t, checkTestCase(ctx, tc, testcase)) // technically we could save the lastplan.toml as an output file and verify here
	require.NoError(t, waitForFile(ctx, tc, "/var/lib/materia/output/plan.toml", true, 30*time.Second))

	require.NoError(t, stopMateriaServer(ctx, tc, testcase), "unable to stop server")
	require.NoError(t, waitForFile(ctx, tc, "/run/materia/materia.sock", false, 30*time.Second))
}

func Test_ServerMode_Sync(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := serverModeSync
	trackServices(testcase)

	require.NoError(t, runMateriaServer(ctx, tc, testcase), "materia server failed to start")
	require.NoError(t, waitForFile(ctx, tc, "/run/materia/materia.sock", true, 30*time.Second))
	require.NoError(t, waitForFile(ctx, tc, "/var/lib/materia/output/lastrun.toml", true, 30*time.Second))

	require.NoError(t, checkTestCase(ctx, tc, testcase))

	require.NoError(t, stopMateriaServer(ctx, tc, testcase), "unable to stop server")
	require.NoError(t, waitForFile(ctx, tc, "/run/materia/materia.sock", false, 30*time.Second))
}

func Test_ServerMode_Agent(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc, false))
	testcase := serverModeAgent
	trackServices(testcase)

	require.NoError(t, runMateriaServer(ctx, tc, testcase), "materia server failed to start")
	require.NoError(t, waitForFile(ctx, tc, "/run/materia/materia.sock", true, 30*time.Second))

	require.NoError(t, checkTestCase(ctx, tc, testcase))

	// TODO actually validate agent command output
	require.NoError(t, runMateriaCmd(ctx, tc, "agent", "facts"), "facts failed")
	require.NoError(t, runMateriaCmd(ctx, tc, "agent", "sync"), "sync failed")
	require.NoError(t, runMateriaCmd(ctx, tc, "agent", "plan"), "plan failed")
	require.NoError(t, runMateriaCmd(ctx, tc, "agent", "update"), "update failed")
	require.NoError(t, runMateriaCmd(ctx, tc, "agent", "facts"), "facts failed")

	require.NoError(t, stopMateriaServer(ctx, tc, testcase), "unable to stop server")
	require.NoError(t, waitForFile(ctx, tc, "/run/materia/materia.sock", false, 30*time.Second))
}
