package main

import (
	"fmt"
	"slices"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"primamateria.systems/materia/internal/attributes"
	"primamateria.systems/materia/pkg/manifests"
)

var testcases = []TestCase{
	simpleRepo,
	simpleRepo2,
	updatedRes1,
	updatedRes2,
	plannerConfigs,
	ensureQuadlets,
	appMode,
	quadletDropins,
	migration1,
	migration2,
	sopsTest,
	allResources,
	componentScripts,
	instancedComponents,
	containerWithBuild,

	exampleRepo,
	exampleRepoBranch,

	ociSource,

	rollbackGitFailed,
	rollbackGitSuccess,
	rollbackOciFailed,
	rollbackOciSuccess,
}

var simpleRepo = TestCase{
	Name:   "simple-repo",
	Config: defaultConfig("simple-repo"),
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

var updatedRes1 = TestCase{
	Name:   "updated-res-1",
	Config: defaultConfig("updated-res-1"),
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

var updatedRes2 = TestCase{
	Name:   "updated-res-2",
	Config: defaultConfig("updated-res-2"),
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

var plannerConfigs = TestCase{
	Name: "planner-configs",
	Config: mustConfig("planner-configs", map[string]any{
		"hostname":                 "localhost",
		"quiet":                    "true",
		"file.base_dir":            "attributes",
		"planner.cleanup_quadlets": "true",
		"planner.backup_volumes":   "false",
		"source.kind":              "local",
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

var ensureQuadlets = TestCase{
	Name: "ensure-quadlets",
	Config: mustConfig("ensure-quadlets", map[string]any{
		"hostname":      "localhost",
		"quiet":         "true",
		"file.base_dir": "attributes",
		"source.kind":   "local",
		"source.url":    "file:///root/tests/ensure-quadlets/source",
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

var quadletDropins = TestCase{
	Name: "quadlet-dropins",
	Config: mustConfig("quadlet-dropins", map[string]any{
		"hostname":      "localhost",
		"quiet":         "true",
		"file.base_dir": "attributes",
		"source.kind":   "local",
		"source.url":    "file:///root/tests/quadlet-dropins/source",
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

var appMode = TestCase{
	Name: "app-mode",
	Config: mustConfig("app-mode", map[string]any{
		"hostname":      "localhost",
		"quiet":         "true",
		"appmode":       "true",
		"file.base_dir": "attributes",
		"source.kind":   "local",
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

var simpleRepo2 = func() TestCase {
	tc := TestCase{
		Name:   "simple-repo-2",
		Config: defaultConfig("simple-repo-2"),
		Source: TestRepo{
			Manifest: &manifests.MateriaManifest{
				Hosts: map[string]manifests.Host{
					"localhost": {
						Components: []string{"carpal", "freshrss"},
						Roles:      []string{"double"},
					},
				},
				Roles: map[string]manifests.Role{
					"double": {Components: []string{"double"}},
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
	injectComponentAttribute(tc.Source.Attributes["vault.toml"], "carpal", "configContents", `
driver: file
file:
  directory: /etc/carpal/resources/`)
	injectComponentAttribute(tc.Source.Attributes["vault.toml"], "carpal", "ldapTemplate", `
  aliases:
    - "mailto:{{ index . "mail" }}"`)

	injectComponentAttribute(tc.Source.Attributes["vault.toml"], "freshrss", "domain", "rss.example.com")
	injectComponentAttribute(tc.Source.Attributes["vault.toml"], "freshrss", "cron", "1,31")
	injectComponentAttribute(tc.Source.Attributes["vault.toml"], "freshrss", "timezone", "America/NewYork")
	return tc
}()

var helloWithNetworkVolume = TestComponent{
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
			Path:    "MANIFEST.toml",
			Content: "\n\t\t\t[[Services]]\n\t\t\tService = \"hello.container\"\n\t\t\t",
		},
		{Path: "hello.volume", Content: "[Volume]\nLabel=foo=bar\n"},
		{Path: "hello.network", Content: "[Network]\n"},
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

var migration1 = TestCase{
	Name: "migration-1",
	Config: mustConfig("migration-1", map[string]any{
		"hostname": "localhost", "quiet": "true", "file.base_dir": "attributes",
		"planner.migrate_volumes": "true", "source.kind": "local",
		"source.url": "file:///root/tests/migration-1/source",
	}),
	Source: TestRepo{Manifest: defaultManifest("hello"), Components: []TestComponent{helloQuadlets}},
	Output: TestOutput{
		ActiveServices: []string{"hello.service"},
		Components:     []string{"hello"},
		Files:          helloQuadlets.Output,
	},
}

var migration2 = TestCase{
	Name: "migration-2",
	Config: mustConfig("migration-2", map[string]any{
		"hostname": "localhost", "file.base_dir": "attributes",
		"planner.migrate_volumes": "true", "source.kind": "local",
		"source.url": "file:///root/tests/migration-2/source",
	}),
	Source: TestRepo{Manifest: defaultManifest("hello"), Components: []TestComponent{helloWithNetworkVolume}},
	Output: TestOutput{
		ActiveServices: []string{"hello.service"},
		Components:     []string{"hello"},
		Files:          helloWithNetworkVolume.Output,
	},
}

var sopsTest = func() TestCase {
	tc := TestCase{
		Name: "sops-test",
		Config: mustConfig("sops-test", map[string]any{
			"hostname":      "localhost",
			"quiet":         "true",
			"sops.base_dir": "attributes",
			"sops.suffix":   "enc",
			"source.kind":   "local",
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

	injectComponentAttribute(tc.Source.Attributes["vault.yml"], "hello", "containerTag", "latest")
	return tc
}()

var allResources = TestCase{
	Name:   "all-resources",
	Config: defaultConfig("all-resources"),
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

var componentScripts = TestCase{
	Name:   "component-scripts",
	Config: defaultConfig("component-scripts"),
	Source: TestRepo{Manifest: defaultManifest("hello"), Components: []TestComponent{helloWithScripts}},
	Output: TestOutput{
		Components: []string{"hello"},
		Files:      helloWithScripts.Output,
	},
}

var containerWithBuild = TestCase{
	Name:   "container-with-build",
	Config: defaultConfig("container-with-build"),
	Source: TestRepo{
		AttributesKind: "sops",
		Manifest:       defaultManifest("hello"),
		Components:     []TestComponent{helloBuild},
	},
	Output: TestOutput{
		ActiveServices:   []string{"hello.service"},
		InactiveServices: []string{"hello-build.service"},
		Components:       []string{"hello"},
		Files:            helloBuild.Output,
	},
}

var instancedComponents = func() TestCase {
	tc := TestCase{
		Name:   "instanced-components",
		Config: defaultConfig("instanced-components"),
		Source: TestRepo{
			AttributesKind: "file",
			Manifest:       defaultManifest("hello@foo", "hello@bar"),
			Components:     []TestComponent{helloInstanced},
			Attributes: map[string]attributes.AttributeVault{
				"vault.toml": {Components: map[string]map[string]any{}},
			},
		},
		Output: TestOutput{
			ActiveServices: []string{"hello@foo.service", "hello@bar.service"},
			Components:     []string{"hello@foo", "hello@bar"},
			Files:          helloInstanced.Output,
		},
	}
	injectComponentAttribute(tc.Source.Attributes["vault.toml"], "hello", "containerTag", "latest")
	injectComponentAttribute(tc.Source.Attributes["vault.toml"], "hello@foo", "mountPoint", "hellofoo")
	injectComponentAttribute(tc.Source.Attributes["vault.toml"], "hello@bar", "mountPoint", "hellobar")
	return tc
}()

var exampleRepo = TestCase{
	Name: "example-repo",
	Config: mustConfig("example-repo", map[string]any{
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

var exampleRepoBranch = TestCase{
	Name: "example-repo-branch",
	Config: mustConfig("example-repo-branch", map[string]any{
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

var ociSource = TestCase{
	Name: "example-repo-oci",
	Config: mustConfig("oci-source", map[string]any{
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

var rollbackGitFailed = TestCase{
	Name: "rollback-git-failed",
	Config: mustConfig("rollback-git-failed", map[string]any{
		"hostname":      "localhost",
		"quiet":         "true",
		"sops.base_dir": "attributes",
		"sops.suffix":   "enc",
		"source.kind":   "git",
		"source.url":    "/tmp/materia/repo",
	}),

	Source: TestRepo{Remote: true},
	Output: TestOutput{
		ActiveServices:   []string{"freshrss.service", "podman_exporter.service"},
		InactiveServices: []string{},
		Components:       []string{"freshrss", "podman_exporter"},
		Files:            slices.Concat(exampleRepoFreshRSSOutput, exampleRepoPodmanExporterOutput),
	},
}

var rollbackGitSuccess = TestCase{
	Name: "rollback-git-success",
	Config: mustConfig("rollback-git-success", map[string]any{
		"hostname":      "localhost",
		"quiet":         "true",
		"sops.base_dir": "attributes",
		"sops.suffix":   "enc",
		"source.kind":   "git",
		"source.url":    "/tmp/materia/repo",
		"rollback.kind": "service",
	}),

	Source: TestRepo{Remote: true},
	Output: TestOutput{
		ActiveServices:   []string{"freshrss.service", "podman_exporter.service"},
		InactiveServices: []string{},
		Components:       []string{"freshrss", "podman_exporter"},
		Files:            slices.Concat(exampleRepoFreshRSSOutput, exampleRepoPodmanExporterOutput),
	},
}

var rollbackOciFailed = TestCase{
	Name: "rollback-oci-failed",
	Config: mustConfig("rollback-oci-failed", map[string]any{
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

var rollbackOciSuccess = TestCase{
	Name: "rollback-oci-success",
	Config: mustConfig("rollback-oci-success", map[string]any{
		"hostname":      "localhost",
		"quiet":         "true",
		"sops.base_dir": "attributes",
		"sops.suffix":   "enc",
		"source.kind":   "oci",
		"source.url":    "oci://git.saintnet.tech/stryan/materia-example-repo:latest",
		"rollback.kind": "service",
	}),
	Source: TestRepo{Remote: true},
	Output: TestOutput{
		ActiveServices:   []string{"freshrss.service", "podman_exporter.service"},
		InactiveServices: []string{},
		Components:       []string{"freshrss", "podman_exporter"},
		Files:            slices.Concat(exampleRepoFreshRSSOutput, exampleRepoPodmanExporterOutput),
	},
}

func newConfig(input map[string]any) (*koanf.Koanf, error) {
	k := koanf.New(".")
	if err := k.Load(confmap.Provider(input, "."), nil); err != nil {
		return nil, err
	}
	return k, nil
}

func mustConfig(name string, input map[string]any) *koanf.Koanf {
	k, err := newConfig(input)
	if err != nil {
		panic(fmt.Sprintf("building config for fixture %v: %v", name, err))
	}
	return k
}

func defaultConfig(name string) *koanf.Koanf {
	return mustConfig(name, map[string]any{
		"hostname":      "localhost",
		"quiet":         "true",
		"file.base_dir": "attributes",
		"source.kind":   "local",
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
