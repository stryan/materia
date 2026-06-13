package main

var hello = TestComponent{
	Name: "hello",
	Files: []TestFile{
		{
			Path:    "hello.container",
			Content: "[Container]\nImage=docker.io/busybox:stable\n",
		},
		{
			Path: "MANIFEST.toml",
		},
	},
	Output: []TestFile{
		{
			Path:    "/etc/containers/systemd/hello/hello.container",
			Content: "[Container]\nImage=docker.io/busybox:stable\n",
		},
		{
			Path: "/var/lib/materia/components/hello/MANIFEST.toml",
		},
	},
}

var helloTmpl = TestComponent{
	Name: "hello",
	Files: []TestFile{
		{
			Path:    "hello.container",
			Content: "[Container]\nImage=docker.io/busybox:{{.containerTag}}\n",
		},
		{
			Path: "MANIFEST.toml",
		},
	},
	Output: []TestFile{
		{
			Path:    "/etc/containers/systemd/hello/hello.container",
			Content: "[Container]\nImage=docker.io/busybox:stable\n",
		},
		{
			Path: "/var/lib/materia/components/hello/MANIFEST.toml",
		},
	},
}

var helloQuadlets = TestComponent{
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
			Content: "[Volume]\n",
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
			Content: "[Volume]\n",
		},
		{
			Path:    "/etc/containers/systemd/hello/hello.network",
			Content: "[Network]\n",
		},
	},
}

var double = TestComponent{
	Name: "double",
	Files: []TestFile{
		{
			Path:    "foo.container",
			Content: "[Container]\nImage=docker.io/busybox:stable\n",
		},
		{
			Path:    "bar.container",
			Content: "[Container]\nImage=docker.io/busybox:stable\n",
		},
		{
			Path: "MANIFEST.toml",
			Content: `
			[[Services]]
			Service = "foo.container"
			Oneshot = true
			[[Services]]
			Service = "bar.service"
			Oneshot = true
			`,
		},
	},
	Output: []TestFile{
		{
			Path:    "/etc/containers/systemd/double/foo.container",
			Content: "[Container]\nImage=docker.io/busybox:stable\n",
		},
		{
			Path:    "/etc/containers/systemd/double/bar.container",
			Content: "[Container]\nImage=docker.io/busybox:stable\n",
		},
		{
			Path: "/var/lib/materia/components/double/MANIFEST.toml",
			Content: `
			[[Services]]
			Service = "foo.container"
			Oneshot = true
			[[Services]]
			Service = "bar.service"
			Oneshot = true
			`,
		},
	},
}

var freshRssTmpl = TestComponent{
	Name: "freshrss",
	Files: []TestFile{
		{
			Path: "MANIFEST.toml",
			Content: `
				Defaults.containerTag = "latest"
				Defaults.Port = 80
				Secrets = ["domain"]

				[[Services]]
				Service = "freshrss.service"
			`,
		},
		{
			Path:    "freshrss-data.volume",
			Content: "[Volume]\n",
		},
		{
			Path:    "freshrss-extensions.volume",
			Content: "[Volume]\n",
		},
		{
			Path: "freshrss.container.gotmpl",
			Content: `
			[Container]
			Image=docker.io/freshrss/freshrss:{{.containerTag}}
			ContainerName=freshrss
			EnvironmentFile={{ m_dataDir "freshrss" }}/freshrss.env
			Volume=freshrss-data.volume:/var/www/FreshRSS/data
			Volume=freshrss-extensions.volume:/var/www/FreshRSS/extensions
			PublishPort={{ .Port }}:80
			{{ secretEnv "domain" "SERVER_DNS" }}
			`,
		},
		{
			Path:    "freshrss.env.gotmpl",
			Content: "TZ={{.timezone}}\nCRON_MIN={{.cron}}",
		},
	},
	Output: []TestFile{
		{
			Path: "/var/lib/materia/components/freshrss/MANIFEST.toml",
			Content: `
			Defaults.containerTag = "latest"
			Defaults.Port = 80
			Secrets = ["domain"]

			[[Services]]
			Service = "freshrss.service"
			`,
		},
		{
			Path: "/var/lib/materia/components/freshrss/freshrss.env",
			Content: `
			TZ=America/NewYork
			CRON_MIN=1,31
			`,
		},
		{
			Path:    "/etc/containers/systemd/freshrss/freshrss-data.volume",
			Content: "[Volume]\n",
		},
		{
			Path:    "/etc/containers/systemd/freshrss/freshrss-extensions.volume",
			Content: "[Volume]\n",
		},
		{
			Path: "/etc/containers/systemd/freshrss/freshrss.container",
			Content: `
			[Container]
			Image=docker.io/freshrss/freshrss:latest
			ContainerName=freshrss
			EnvironmentFile=/var/lib/materia/components/freshrss/freshrss.env
			Volume=freshrss-data.volume:/var/www/FreshRSS/data
			Volume=freshrss-extensions.volume:/var/www/FreshRSS/extensions
			PublishPort=80:80
			Secret=materia-domain,type=env,target=SERVER_DNS
			`,
		},
	},
}

var carpalTmpl = TestComponent{
	Name: "carpal",
	Files: []TestFile{
		{
			Path: "MANIFEST.toml",
			Content: `
			[Defaults]
			port = 8000
			containerTag = "latest"

			[[Services]]
			Service = "carpal.service"
			ReloadedBy = ["conf/config.yml","conf/ldap.yml"]
			RestartedBy = ["conf/config.yml","carpal.container"]
			`,
		},
		{
			Path: "carpal.container.gotmpl",
			Content: `
			[Unit]
			Description=carpal container
			After=local-fs.target network.target
			StartLimitIntervalSec=300
			StartLimitBurst=5

			[Service]
			SuccessExitStatus=2


			[Container]
			Image=docker.io/peeley/carpal:{{.containerTag}}
			ContainerName=carpal
			Volume={{ m_dataDir "carpal" }}/conf:/etc/carpal:Z
			PublishPort={{.port}}:8008

			[Install]
			# Start by default on boot
			WantedBy=multi-user.target default.target
			`,
		},
		{
			Path:    "conf/config.yml.gotmpl",
			Content: "{{.configContents}}",
		},
		{
			Path:    "conf/ldap.gotmpl.gotmpl",
			Content: "{{ .ldapTemplate }}",
		},
		{
			Path:  "conf/resources/",
			IsDir: true,
		},
	},
	Output: []TestFile{
		{
			Path: "/var/lib/materia/components/carpal/MANIFEST.toml",
			Content: `
			[Defaults]
			port = 8000
			containerTag = "latest"

			[[Services]]
			Service = "carpal.service"
			ReloadedBy = ["conf/config.yml","conf/ldap.yml"]
			RestartedBy = ["conf/config.yml","carpal.container"]
			`,
		},
		{
			Path: "/etc/containers/systemd/carpal/carpal.container",
			Content: `
			[Unit]
			Description=carpal container
			After=local-fs.target network.target
			StartLimitIntervalSec=300
			StartLimitBurst=5

			[Service]
			SuccessExitStatus=2


			[Container]
			Image=docker.io/peeley/carpal:latest
			ContainerName=carpal
			Volume=/var/lib/materia/components/carpal/conf:/etc/carpal:Z
			PublishPort=8008:8008

			[Install]
			# Start by default on boot
			WantedBy=multi-user.target default.target
			`,
		},
		{
			Path:    "/var/lib/materia/components/carpal/conf/config.yml.gotmpl",
			Content: "{{.configContents}}",
		},
		{
			Path:    "/var/lib/materia/components/carpal/conf/ldap.gotmpl.gotmpl",
			Content: "{{ .ldapTemplate }}",
		},
		{
			Path:  "/var/lib/materia/components/carpal/conf/resources/",
			IsDir: true,
		},
	},
}

var exampleRepoFreshRSSOutput = []TestFile{
	{
		Path: "/var/lib/materia/components/freshrss/MANIFEST.toml",
		Content: `
			Defaults.containerTag = "latest"
			Defaults.Port = 80
			Secrets = ["domain"]

			[[Services]]
			Service = "freshrss.service"
			`,
	},
	{
		Path: "/var/lib/materia/components/freshrss/freshrss.env",
		Content: `
			TZ=America/NewYork
			CRON_MIN=1,31
			`,
	},
	{
		Path:    "/etc/containers/systemd/freshrss/freshrss-data.volume",
		Content: "[Volume]\n",
	},
	{
		Path:    "/etc/containers/systemd/freshrss/freshrss-extensions.volume",
		Content: "[Volume]\n",
	},
	{
		Path: "/etc/containers/systemd/freshrss/freshrss.container",
		Content: `
			[Unit]
			Description=FreshRSS container
			StartLimitIntervalSec=300
			StartLimitBurst=5


			[Service]
			Restart=on-failure
			RestartSec=5s


			[Container]
			Image=docker.io/freshrss/freshrss:latest
			ContainerName=freshrss
			EnvironmentFile=/var/lib/materia/components/freshrss/freshrss.env
			Volume=freshrss-data.volume:/var/www/FreshRSS/data
			Volume=freshrss-extensions.volume:/var/www/FreshRSS/extensions
			PublishPort=80:80
			Secret=materia-domain,type=env,target=SERVER_DNS

			[Install]
			# Start by default on boot
			WantedBy=multi-user.target default.target
			`,
	},
}

var exampleRepoPodmanExporterOutput = []TestFile{
	{
		Path: "/etc/containers/systemd/podman_exporter/podman_exporter.container",
		Content: `[Unit]
			Description=Podman prometheus exporter


			[Service]
			Restart=on-failure
			RestartSec=5s

			[Container]
			Image=quay.io/navidys/prometheus-podman-exporter:latest
			ContainerName=podman_exporter
			Volume=/run/podman/podman.sock:/run/podman/podman.sock
			Environment=CONTAINER_HOST=unix:///run/podman/podman.sock
			SecurityLabelDisable=true
			User=root
			PublishPort=9882:9882

			[Install]
			# Start by default on boot
			WantedBy=multi-user.target default.target
		`,
	},
	{
		Path: "/var/lib/materia/components/podman_exporter/MANIFEST.toml",
		Content: `
			Defaults.containerTag = "latest"
			Defaults.Port = 9882

			[[Services]]
			Service = "podman_exporter.service"
			RestartedBy = ["podman_exporter.container"]
			`,
	},
}
