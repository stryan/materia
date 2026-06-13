package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"charm.land/log/v2"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"primamateria.systems/materia/internal/attributes"
	"primamateria.systems/materia/pkg/manifests"
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
	require.NoError(t, reset(ctx, tc))
	testcase := TestCase{
		Name:   "simple-repo",
		Config: defaultConfig(t, "simple-repo"),
		Source: TestRepo{
			Manifest:   defaultManifest("hello"),
			Components: []TestComponent{hello},
		},
		Output: TestOutput{
			ActiveServices:   []string{},
			InactiveServices: []string{},
			Components:       []string{"hello"},
			Files:            hello.Output,
		},
	}
	trackServices(testcase)
	require.NoError(t, installTestCase(ctx, tc, testcase))
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
	require.NoError(t, reset(ctx, tc))
	testcase := TestCase{
		Name:   "simple-repo-2",
		Config: defaultConfig(t, "simple-repo-2"),
		Source: TestRepo{
			Manifest: &manifests.MateriaManifest{
				Hosts: map[string]manifests.Host{
					"localhost": {
						Components: []string{"carpal", "freshrss"},
						Roles:      []string{"double"},
					},
				},
				Roles: map[string]manifests.Role{
					"double": {
						Components: []string{"double"},
					},
				},
			},
			Components: []TestComponent{carpalTmpl, freshRssTmpl, double},
			Attributes: map[string]attributes.AttributeVault{
				"vault.toml": {
					Components: map[string]map[string]any{},
				},
			},
		},
		Output: TestOutput{
			ActiveServices:   []string{"freshrss.service", "carpal.service", "foo.service", "bar.service"},
			InactiveServices: []string{},
			Components:       []string{"freshrss", "carpal", "double"},
			Files:            slices.Concat(freshRssTmpl.Output, carpalTmpl.Output, double.Output),
		},
	}
	trackServices(testcase)
	injectComponentAttribute(testcase.Source.Attributes["vault.toml"], "carpal", "configContents", `
driver: file
file:
  directory: /etc/carpal/resources/`)

	injectComponentAttribute(testcase.Source.Attributes["vault.toml"], "carpal", "ldapTemplate", `
  aliases:
    - "mailto:{{ index . "mail" }}"
  links:
    - rel: "http://openid.net/specs/connect/1.0/issuer"
      href: "https://login.foobar.com"`)

	injectComponentAttribute(testcase.Source.Attributes["vault.toml"], "freshrss", "domain", "rss.example.com")
	injectComponentAttribute(testcase.Source.Attributes["vault.toml"], "freshrss", "cron", "1,31")
	injectComponentAttribute(testcase.Source.Attributes["vault.toml"], "freshrss", "timezone", "America/NewYork")
	require.NoError(t, installTestCase(ctx, tc, testcase))
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "plan"))
	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
}

func Test_Sops(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc))
	testcase := TestCase{
		Name: "sops-test",
		Config: newConfig(t, map[string]any{
			"hostname":      "localhost",
			"quiet":         "true",
			"sops.base_dir": "attributes",
			"sops.suffix":   "enc",
			"source.kind":   "file",
			"source.url":    fmt.Sprintf("file:///root/tests/%v/source", "sops-test"),
		}),
		Source: TestRepo{
			AttributesKind: "sops",
			Manifest:       defaultManifest("hello"),
			Components:     []TestComponent{helloTmpl},
			Attributes: map[string]attributes.AttributeVault{
				"vault.yml": {
					Components: map[string]map[string]any{},
				},
			},
		},
		Output: TestOutput{
			ActiveServices:   []string{},
			InactiveServices: []string{},
			Components:       []string{"hello"},
			Files: []TestFile{
				{
					Path:    "/etc/containers/systemd/hello/hello.container",
					Content: "[Container]\nImage=docker.io/busybox:latest\n",
				},
				{
					Path: "/var/lib/materia/components/hello/MANIFEST.toml",
				},
			},
		},
	}
	trackServices(testcase)
	injectComponentAttribute(testcase.Source.Attributes["vault.yml"], "hello", "containerTag", "latest")
	require.NoError(t, testcase.Setup())
	require.NoError(t, installTestCase(ctx, tc, testcase))
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, setEnv(ctx, tc, "SOPS_AGE_KEY_FILE", filepath.Join(testcase.Destination(), "config", "key.txt")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_VolumeMigration(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc))
	testcase1 := TestCase{
		Name: "migration-1",
		Config: newConfig(t, map[string]any{
			"hostname":                "localhost",
			"quiet":                   "true",
			"file.base_dir":           "attributes",
			"planner.migrate_volumes": "true",
			"source.kind":             "file",
			"source.url":              fmt.Sprintf("file:///root/tests/%v/source", "migration-1"),
		}),
		Source: TestRepo{
			Manifest:   defaultManifest("hello"),
			Components: []TestComponent{helloQuadlets},
		},
		Output: TestOutput{
			ActiveServices:   []string{"hello.service"},
			InactiveServices: []string{},
			Components:       []string{"hello"},
			Files:            helloQuadlets.Output,
		},
	}
	comp2 := TestComponent{
		Name: "hello",
		Files: []TestFile{
			{
				Path: "hello.container",
				Content: `
			[Unit]
			Description=Hello Service
			Wants=network-online.target
			After=network-online.target

			[Container]
			ContainerName=busybox1
			Image=docker.io/busybox:latest
			Exec=/bin/sh -c "trap 'exit 0' INT TERM; while true; do echo Hello World; sleep 1; done"
			Network=hello.network
			Volume=hello.volume:/hello

			[Install]
			WantedBy=multi-user.target
			`,
			},
			{
				Path: "MANIFEST.toml",
				Content: `
			[[Services]]
			Service = "hello.container"
			`,
			},
			{
				Path:    "hello.volume",
				Content: "[Volume]\nLabel=foo=bar\n",
			},
			{
				Path:    "hello.network",
				Content: "[Network]\n",
			},
		},
		Output: []TestFile{
			{
				Path: "/etc/containers/systemd/hello/hello.container",
				Content: `
			[Unit]
			Description=Hello Service
			Wants=network-online.target
			After=network-online.target

			[Container]
			ContainerName=busybox1
			Image=docker.io/busybox:latest
			Exec=/bin/sh -c "trap 'exit 0' INT TERM; while true; do echo Hello World; sleep 1; done"
			Network=hello.network
			Volume=hello.volume:/hello

			[Install]
			WantedBy=multi-user.target
			`,
			},
			{
				Path: "/var/lib/materia/components/hello/MANIFEST.toml",
				Content: `
			[[Services]]
			Service = "hello.container"
			`,
			},
			{
				Path:    "/etc/containers/systemd/hello/hello.volume",
				Content: "[Volume]\nLabel=foo=bar\n",
			},
			{
				Path:    "/etc/containers/systemd/hello/hello.network",
				Content: "[Network]\n",
			},
		},
	}
	testcase2 := TestCase{
		Name: "migration-2",
		Config: newConfig(t, map[string]any{
			"hostname": "localhost",
			// "quiet":                   "true",
			"file.base_dir":           "attributes",
			"planner.migrate_volumes": "true",
			"source.kind":             "file",
			"source.url":              fmt.Sprintf("file:///root/tests/%v/source", "migration-2"),
		}),
		Source: TestRepo{
			Manifest:   defaultManifest("hello"),
			Components: []TestComponent{comp2},
		},
		Output: TestOutput{
			ActiveServices:   []string{"hello.service"},
			InactiveServices: []string{},
			Components:       []string{"hello"},
			Files:            comp2.Output,
		},
	}
	trackServices(testcase2)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase1.Destination(), "config", "config.toml")))
	require.NoError(t, testcase1.Setup())
	require.NoError(t, testcase2.Setup())
	require.NoError(t, installTestCase(ctx, tc, testcase1, testcase2))

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
	require.NoError(t, reset(ctx, tc))
	testcase := TestCase{
		Name: "example-repo",
		Config: newConfig(t, map[string]any{
			"hostname":      "localhost",
			"quiet":         "true",
			"sops.base_dir": "attributes",
			"sops.suffix":   "enc",
			"source.kind":   "git",
			"source.url":    "https://github.com/stryan/materia_example_repo",
		}),
		Source: TestRepo{Remote: true},
		Output: TestOutput{
			ActiveServices:   []string{"freshrss.service", "podman_exporter.service"},
			InactiveServices: []string{},
			Components:       []string{"freshrss", "podman_exporter"},
			Files:            slices.Concat(exampleRepoFreshRSSOutput, exampleRepoPodmanExporterOutput),
		},
	}
	trackServices(testcase)
	require.NoError(t, testcase.Setup())
	require.NoError(t, installTestCase(ctx, tc, testcase))
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, setEnv(ctx, tc, "SOPS_AGE_KEY_FILE", "/var/lib/materia/source/key.txt"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_ExampleRepoBranch(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc))
	testcase := TestCase{
		Name: "example-repo-branch",
		Config: newConfig(t, map[string]any{
			"hostname":      "localhost",
			"quiet":         "true",
			"sops.base_dir": "attributes",
			"sops.suffix":   "enc",
			"source.kind":   "git",
			"source.url":    "https://github.com/stryan/materia_example_repo",
		}),
		Source: TestRepo{Remote: true},
		Output: TestOutput{
			ActiveServices:   []string{"freshrss.service", "podman_exporter.service"},
			InactiveServices: []string{},
			Components:       []string{"freshrss", "podman_exporter"},
			Files:            slices.Concat(exampleRepoFreshRSSOutput, exampleRepoPodmanExporterOutput),
		},
	}
	trackServices(testcase)
	require.NoError(t, testcase.Setup())
	require.NoError(t, installTestCase(ctx, tc, testcase))
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

func Test_AllResources(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc))
	helloAll := TestComponent{
		Name: "hello-all",
		Files: []TestFile{
			{
				Path:    "Containerfile",
				Content: "FROM busybox\nCOPY /var/lib/materia/components/hello-all/test.env /test.env",
			},
			{
				Path: "MANIFEST.toml",
			},
			{
				Path:    "busybox.image",
				Content: "[Image]\nImageTag=docker.io/busybox:latest\nImage=docker.io/busybox:latest",
			},
			{
				Path:    "hello.build",
				Content: "[Build]\nImageTag=localhost/custombusybox:latest\nFile=/var/lib/materia/components/hello-all/Containerfile",
			},
			{
				Path:    "hello.container",
				Content: "[Container]\nImage=busybox.image\n",
			},
			{
				Path:    "hello.kube",
				Content: "[Kube]\nYaml=/var/lib/materia/components/hello-all/hello.yaml",
			},
			{
				Path:    "hello.network",
				Content: "[Network]\n",
			},
			{
				Path:    "hello.sh",
				Content: "#!/bin/bash\necho 'Hello world'\n",
			},
			{
				Path:    "hello.volume",
				Content: "[Volume]\n",
			},
			{
				Path: "hello.yaml",
				Content: `apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2021-09-20T17:40:19Z"
  labels:
	app: php
  name: php
spec:
  containers:
  - args:
	- apache2-foreground
	command:
	- docker-php-entrypoint
	env:
	- name: PATH
  	value: /usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
	- name: TERM
  	value: xterm
	- name: container
  	...
	- name: PHP_EXTRA_BUILD_DEPS
  	value: apache2-dev
	- name: APACHE_ENVVARS
  	value: /etc/apache2/envvars
	image: php-7.2-apache-mysqli:latest
	name: apache
	ports:
	- containerPort: 80
  	hostPort: 8080
  	protocol: TCP
	resources: {}
	securityContext:
  	allowPrivilegeEscalation: true
  	capabilities:
    	drop:
    	- CAP_MKNOD
    	- CAP_NET_RAW
    	- CAP_AUDIT_WRITE
  	privileged: false
  	readOnlyRootFilesystem: false
  	seLinuxOptions: {}
	tty: true
	workingDir: /var/www/html
  dnsConfig: {}
  restartPolicy: Never
status: {}`,
			},
			{
				Path:    "hello_world.service",
				Content: "[Unit]\nDescription=Hello\n\n[Service]\nType=oneshot\nExecStart=/usr/local/bin/hello.sh",
			},
			{
				Path:    "test.env",
				Content: "CONFIG=config",
			},
		},
		Output: []TestFile{
			{
				Path:    "/var/lib/materia/components/hello-all/Containerfile",
				Content: "FROM busybox\nCOPY /var/lib/materia/components/hello-all/test.env /test.env",
			},
			{
				Path: "/var/lib/materia/components/hello-all/MANIFEST.toml",
			},
			{
				Path:    "/etc/containers/systemd/hello-all/busybox.image",
				Content: "[Image]\nImageTag=docker.io/busybox:latest\nImage=docker.io/busybox:latest",
			},
			{
				Path:    "/etc/containers/systemd/hello-all/hello.build",
				Content: "[Build]\nImageTag=localhost/custombusybox:latest\nFile=/var/lib/materia/components/hello/Containerfile",
			},
			{
				Path:    "/etc/containers/systemd/hello-all/hello.container",
				Content: "[Container]\nImage=busybox.image\n",
			},
			{
				Path:    "/etc/containers/systemd/hello-all/hello.kube",
				Content: "[Kube]\nYaml=/var/lib/materia/components/hello/hello.yaml",
			},
			{
				Path:    "/etc/containers/systemd/hello-all/hello.network",
				Content: "[Network]\n",
			},
			{
				Path:    "/var/lib/materia/components/hello-all/hello.sh",
				Content: "#!/bin/bash\necho 'Hello world'\n",
			},
			{
				Path:    "/usr/local/bin/hello.sh",
				Content: "#!/bin/bash\necho 'Hello world'\n",
			},
			{
				Path:    "/etc/containers/systemd/hello-all/hello.volume",
				Content: "[Volume\n]",
			},
			{
				Path: "/var/lib/materia/components/hello-all/hello.yaml",
				Content: `apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2021-09-20T17:40:19Z"
  labels:
	app: php
  name: php
spec:
  containers:
  - args:
	- apache2-foreground
	command:
	- docker-php-entrypoint
	env:
	- name: PATH
  	value: /usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
	- name: TERM
  	value: xterm
	- name: container
  	...
	- name: PHP_EXTRA_BUILD_DEPS
  	value: apache2-dev
	- name: APACHE_ENVVARS
  	value: /etc/apache2/envvars
	image: php-7.2-apache-mysqli:latest
	name: apache
	ports:
	- containerPort: 80
  	hostPort: 8080
  	protocol: TCP
	resources: {}
	securityContext:
  	allowPrivilegeEscalation: true
  	capabilities:
    	drop:
    	- CAP_MKNOD
    	- CAP_NET_RAW
    	- CAP_AUDIT_WRITE
  	privileged: false
  	readOnlyRootFilesystem: false
  	seLinuxOptions: {}
	tty: true
	workingDir: /var/www/html
  dnsConfig: {}
  restartPolicy: Never
status: {}`,
			},
			{
				Path:    "/var/lib/materia/components/hello-all/hello_world.service",
				Content: "[Unit]\nDescription=Hello\n\n[Service]\nType=oneshot\nExecStart=/usr/local/bin/hello.sh",
			},
			{
				Path:    "/etc/systemd/system/hello_world.service",
				Content: "[Unit]\nDescription=Hello\n\n[Service]\nType=oneshot\nExecStart=/usr/local/bin/hello.sh",
			},
			{
				Path:    "/var/lib/materia/components/hello-all/test.env",
				Content: "CONFIG=config",
			},
		},
	}
	testcase := TestCase{
		Name:   "all-resources",
		Config: defaultConfig(t, "all-resources"),
		Source: TestRepo{
			Manifest:   defaultManifest("hello-all"),
			Components: []TestComponent{helloAll},
		},
		Output: TestOutput{
			ActiveServices:   []string{},
			InactiveServices: []string{},
			Components:       []string{"hello-all"},
			Files:            helloAll.Output,
		},
	}
	trackServices(testcase)
	require.NoError(t, installTestCase(ctx, tc, testcase))
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_ContainerWithBuild(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc))
	comp := TestComponent{Name: "hello"}
	comp.Files = []TestFile{
		{
			Path: "hello.container",
			Content: `
			[Unit]
			Description=Hello Service
			Wants=network-online.target
			After=network-online.target

			[Container]
			ContainerName=hellobuild
			Image=hello.build
			Exec=/bin/sh -c "trap 'exit 0' INT TERM; while true; do cat /hello; sleep 1; done"

			[Install]
			WantedBy=multi-user.target
			`,
		},
		{
			Path: "MANIFEST.toml",
			Content: `
			[[Services]]
			Service = "hello.container"

			[[Services]]
			Service = "hello.build"
			Stopped = true
			Timeout = 100
			`,
		},
		{
			Path:    "Containerfile",
			Content: "FROM busybox\nRUN echo 'We Built This Container on Rock and Roll' >> /hello",
		},
		{
			Path: "hello.build",
			Content: `
			[Build]
			ImageTag=localhost/hellobuild:latest
			File=/var/lib/materia/components/hello/Containerfile
			`,
		},
	}
	comp.Output = []TestFile{
		{
			Path:    "/etc/containers/systemd/hello/hello.container",
			Content: comp.Files[0].Content,
		},
		{
			Path: "/var/lib/materia/components/hello/MANIFEST.toml",
		},
		{
			Path:    "/var/lib/materia/components/hello/Containerfile",
			Content: comp.Files[2].Content,
		},
		{
			Path:    "/etc/containers/systemd/hello/hello.build",
			Content: comp.Files[3].Content,
		},
	}
	testcase := TestCase{
		Name:   "container-with-build",
		Config: defaultConfig(t, "container-with-build"),
		Source: TestRepo{
			AttributesKind: "sops",
			Manifest:       defaultManifest("hello"),
			Components:     []TestComponent{comp},
		},
		Output: TestOutput{
			ActiveServices:   []string{"hello.service", "hello-build.service"},
			InactiveServices: []string{},
			Components:       []string{"hello"},
			Files:            comp.Output,
		},
	}
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, testcase.Setup())
	require.NoError(t, installTestCase(ctx, tc, testcase))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_PlannerConfigs(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc))
	testcase := TestCase{
		Name: "planner-configs",
		Config: newConfig(t, map[string]any{
			"hostname":                 "localhost",
			"quiet":                    "true",
			"file.base_dir":            "attributes",
			"planner.cleanup_quadlets": "true",
			"planner.backup_volumes":   "false",
			"source.kind":              "file",
			"source.url":               "file:///root/tests/planner-configs/source",
		}),
		Source: TestRepo{
			AttributesKind: "sops",
			Manifest:       defaultManifest("hello"),
			Components:     []TestComponent{helloQuadlets},
		},
		Output: TestOutput{
			ActiveServices:   []string{"hello.service"},
			InactiveServices: []string{},
			Components:       []string{"hello"},
			Files:            helloQuadlets.Output,
		},
	}
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, testcase.Setup())
	require.NoError(t, installTestCase(ctx, tc, testcase))

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
	require.NoError(t, reset(ctx, tc))
	testcase := TestCase{
		Name: "planner-configs",
		Config: newConfig(t, map[string]any{
			"hostname":      "localhost",
			"quiet":         "true",
			"file.base_dir": "attributes",
			"source.kind":   "file",
			"source.url":    "file:///root/tests/planner-configs/source",
		}),
		Source: TestRepo{
			AttributesKind: "sops",
			Manifest:       defaultManifest("hello"),
			Components:     []TestComponent{helloQuadlets},
		},
		Output: TestOutput{
			ActiveServices:   []string{"hello.service"},
			InactiveServices: []string{},
			Components:       []string{"hello"},
			Files:            helloQuadlets.Output,
		},
	}
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, testcase.Setup())
	require.NoError(t, installTestCase(ctx, tc, testcase))

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
	require.NoError(t, reset(ctx, tc))
}

func Test_UpdatedResources(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc))
	testcase1 := TestCase{
		Name:   "updated-res-1",
		Config: defaultConfig(t, "updated-res-1"),
		Source: TestRepo{
			Manifest:   defaultManifest("hello"),
			Components: []TestComponent{helloQuadlets},
		},
		Output: TestOutput{
			ActiveServices:   []string{"hello.service"},
			InactiveServices: []string{},
			Components:       []string{"hello"},
			Files:            helloQuadlets.Output,
		},
	}
	testcase2 := TestCase{
		Name:   "updated-res-2",
		Config: defaultConfig(t, "updated-res-2"),
		Source: TestRepo{
			Manifest:   defaultManifest("hello"),
			Components: []TestComponent{hello},
		},
		Output: TestOutput{
			ActiveServices:   []string{},
			InactiveServices: []string{"hello.service"},
			Components:       []string{"hello"},
			Files:            hello.Output,
		},
	}
	trackServices(testcase2)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase1.Destination(), "config", "config.toml")))
	require.NoError(t, setEnv(ctx, tc, "MATERIA_DEBUG", "1"))
	require.NoError(t, testcase1.Setup())
	require.NoError(t, testcase2.Setup())
	require.NoError(t, installTestCase(ctx, tc, testcase1, testcase2))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
	require.NoError(t, checkTestCase(ctx, tc, testcase1))

	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase2.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))
	require.NoError(t, checkTestCase(ctx, tc, testcase2))
}

func Test_ComponentScripts(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc))
	comp := TestComponent{
		Name: "hello",
		Files: []TestFile{
			{
				Path:    "hello.container",
				Content: "[Container]\nImage=docker.io/busybox:stable\n",
			},
			{
				Path: "MANIFEST.toml",
				Content: `Settings.SetupScript = "setup.sh"
Settings.CleanupScript = "cleanup.sh"`,
			},
			{
				Path:    "setup.sh",
				Content: "#!/bin/bash\ntouch /tmp/hello",
			},
			{
				Path:    "cleanup.sh",
				Content: "#!/bin/bash\nrm /tmp/hello",
			},
		},
		Output: []TestFile{
			{
				Path:    "/etc/containers/systemd/hello/hello.container",
				Content: "[Container]\nImage=docker.io/busybox:stable\n",
			},
			{
				Path: "/var/lib/materia/components/hello/MANIFEST.toml",
				Content: `Settings.SetupScript = "setup.sh"
Settings.CleanupScript = "cleanup.sh"`,
			},
			{
				Path:    "/var/lib/materia/components/hello/setup.sh",
				Content: "#!/bin/bash\ntouch /tmp/hello",
			},
			{
				Path:    "/var/lib/materia/components/hello/cleanup.sh",
				Content: "#!/bin/bash\nrm /tmp/hello",
			},
		},
	}
	testcase := TestCase{
		Name:   "component-scripts",
		Config: defaultConfig(t, "component-scripts"),
		Source: TestRepo{
			Manifest:   defaultManifest("hello"),
			Components: []TestComponent{comp},
		},
		Output: TestOutput{
			ActiveServices:   []string{},
			InactiveServices: []string{},
			Components:       []string{"hello"},
			Files:            comp.Output,
		},
	}
	trackServices(testcase)
	require.NoError(t, installTestCase(ctx, tc, testcase))
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
	require.NoError(t, reset(ctx, tc))
	testcase := TestCase{
		Name: "example-repo-oci",
		Config: newConfig(t, map[string]any{
			"hostname":      "localhost",
			"quiet":         "true",
			"sops.base_dir": "attributes",
			"sops.suffix":   "enc",
			"source.kind":   "oci",
			"source.url":    "oci://git.saintnet.tech/stryan/materia-example-repo:latest",
		}),
		Source: TestRepo{Remote: true},
		Output: TestOutput{
			ActiveServices:   []string{"freshrss.service", "podman_exporter.service"},
			InactiveServices: []string{},
			Components:       []string{"freshrss", "podman_exporter"},
			Files:            slices.Concat(exampleRepoFreshRSSOutput, exampleRepoPodmanExporterOutput),
		},
	}
	trackServices(testcase)
	require.NoError(t, testcase.Setup())
	require.NoError(t, installTestCase(ctx, tc, testcase))
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, setEnv(ctx, tc, "SOPS_AGE_KEY_FILE", "/var/lib/materia/source/key.txt"))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_AppMode(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc))
	testcase := TestCase{
		Name: "app-mode",
		Config: newConfig(t, map[string]any{
			"hostname":      "localhost",
			"quiet":         "true",
			"appmode":       "true",
			"file.base_dir": "attributes",
			"source.kind":   "file",
			"source.url":    fmt.Sprintf("file:///root/tests/%v/source", "app-mode"),
		}),
		Source: TestRepo{
			Manifest:   defaultManifest("hello"),
			Components: []TestComponent{hello},
		},
		Output: TestOutput{
			ActiveServices:   []string{},
			InactiveServices: []string{},
			Components:       []string{"hello"},
			Files: append(hello.Output, TestFile{
				Path:    "/etc/containers/systemd/hello/.hello.app",
				Content: "hello.container",
			}),
		},
	}
	trackServices(testcase)
	require.NoError(t, installTestCase(ctx, tc, testcase))
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func Test_QuadletDropins(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, reset(ctx, tc))
	testcase := TestCase{
		Name: "quadlet-dropins",
		Config: newConfig(t, map[string]any{
			"hostname":      "localhost",
			"quiet":         "true",
			"file.base_dir": "attributes",
			"source.kind":   "file",
			"source.url":    fmt.Sprintf("file:///root/tests/%v/source", "quadlet-dropins"),
		}),
		Source: TestRepo{
			Manifest:   defaultManifest("hello"),
			Components: []TestComponent{helloQuadlets},
		},
		Output: TestOutput{
			ActiveServices:   []string{"hello.service"},
			InactiveServices: []string{},
			Components:       []string{"hello"},
			Files:            helloQuadlets.Output,
		},
	}
	trackServices(testcase)
	require.NoError(t, installTestCase(ctx, tc, testcase))
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
	require.NoError(t, reset(ctx, tc))
	comp := TestComponent{Name: "hello"}
	comp.Files = []TestFile{
		{
			Path: "hello@.container.gotmpl",
			Content: `[Unit]
Description=Hello Service
Wants=network-online.target
After=network-online.target

[Container]
Image=docker.io/busybox:{{.containerTag}}
Exec=/bin/sh -c "trap 'exit 0' INT TERM; while true; do echo Hello World; sleep 1; done"
Volume=hello.volume:/{{.mountPoint}}

[Install]
WantedBy=multi-user.target`,
		},
		{
			Path: "MANIFEST.toml",
			Content: `[[Services]]
			Service = "hello@.container"`,
		},
		{
			Path:    "hello.volume",
			Content: "[Volume]",
		},
	}
	comp.Output = []TestFile{
		{
			Path: "/etc/containers/systemd/hello@foo/hello@foo.container",
			Content: `[Unit]
Description=Hello Service
Wants=network-online.target
After=network-online.target

[Container]
Image=docker.io/busybox:latest
Exec=/bin/sh -c "trap 'exit 0' INT TERM; while true; do echo Hello World; sleep 1; done"
Volume=hello.volume:/hellofoo

[Install]
WantedBy=multi-user.target`,
		},
		{
			Path:    "/var/lib/materia/components/hello@foo/MANIFEST.toml",
			Content: comp.Files[1].Content,
		},
		{
			Path:    "/etc/containers/systemd/hello@foo/hello.volume",
			Content: comp.Files[2].Content,
		},
		{
			Path: "/etc/containers/systemd/hello@bar/hello@bar.container",
			Content: `[Unit]
Description=Hello Service
Wants=network-online.target
After=network-online.target

[Container]
Image=docker.io/busybox:latest
Exec=/bin/sh -c "trap 'exit 0' INT TERM; while true; do echo Hello World; sleep 1; done"
Volume=hello.volume:/hellobar

[Install]
WantedBy=multi-user.target`,
		},
		{
			Path:    "/var/lib/materia/components/hello@bar/MANIFEST.toml",
			Content: comp.Files[1].Content,
		},
		{
			Path:    "/etc/containers/systemd/hello@bar/hello.volume",
			Content: comp.Files[2].Content,
		},
	}
	testcase := TestCase{
		Name:   "instanced-components",
		Config: defaultConfig(t, "instanced-components"),
		Source: TestRepo{
			AttributesKind: "file",
			Manifest:       defaultManifest("hello@foo", "hello@bar"),
			Components:     []TestComponent{comp},
			Attributes: map[string]attributes.AttributeVault{
				"vault.toml": {
					Components: map[string]map[string]any{},
				},
			},
		},
		Output: TestOutput{
			ActiveServices:   []string{"hello@foo.service", "hello@bar.service"},
			InactiveServices: []string{},
			Components:       []string{"hello@foo", "hello@bar"},
			Files:            comp.Output,
		},
	}
	injectComponentAttribute(testcase.Source.Attributes["vault.toml"], "hello", "containerTag", "latest")
	injectComponentAttribute(testcase.Source.Attributes["vault.toml"], "hello@foo", "mountPoint", "hellofoo")
	injectComponentAttribute(testcase.Source.Attributes["vault.toml"], "hello@bar", "mountPoint", "hellobar")
	trackServices(testcase)
	require.NoError(t, setEnv(ctx, tc, "MATERIA_CONFIG", filepath.Join(testcase.Destination(), "config", "config.toml")))
	require.NoError(t, testcase.Setup())
	require.NoError(t, installTestCase(ctx, tc, testcase))

	require.NoError(t, runMateriaCmd(ctx, tc, "update"))

	require.NoError(t, checkTestCase(ctx, tc, testcase))
}

func newConfig(t *testing.T, input map[string]any) *koanf.Koanf {
	k := koanf.New(".")
	require.NoError(t, k.Load(confmap.Provider(input, "."), nil))
	return k
}

func defaultConfig(t *testing.T, name string) *koanf.Koanf {
	return newConfig(t, map[string]any{
		"hostname":      "localhost",
		"quiet":         "true",
		"file.base_dir": "attributes",
		"source.kind":   "file",
		"source.url":    fmt.Sprintf("file:///root/tests/%v/source", name),
		"lock":          "true",
	})
}

func defaultManifest(comps ...string) *manifests.MateriaManifest {
	return &manifests.MateriaManifest{
		Hosts: map[string]manifests.Host{
			"localhost": {
				Components: comps,
			},
		},
	}
}
