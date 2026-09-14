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

var helloWithScripts = TestComponent{
	Name: "hello",
	Files: []TestFile{
		{Path: "hello.container", Content: "[Container]\nImage=docker.io/busybox:stable\n"},
		{Path: "MANIFEST.toml", Content: "Settings.SetupScript = \"setup.sh\"\nSettings.CleanupScript = \"cleanup.sh\""},
		{Path: "setup.sh", Content: "#!/bin/bash\ntouch /tmp/hello"},
		{Path: "cleanup.sh", Content: "#!/bin/bash\nrm /tmp/hello"},
	},
	Output: []TestFile{
		{Path: "/etc/containers/systemd/hello/hello.container", Content: "[Container]\nImage=docker.io/busybox:stable\n"},
		{Path: "/var/lib/materia/components/hello/MANIFEST.toml", Content: "Settings.SetupScript = \"setup.sh\"\nSettings.CleanupScript = \"cleanup.sh\""},
		{Path: "/var/lib/materia/components/hello/setup.sh", Content: "#!/bin/bash\ntouch /tmp/hello"},
		{Path: "/var/lib/materia/components/hello/cleanup.sh", Content: "#!/bin/bash\nrm /tmp/hello"},
	},
}

var helloBuild = func() TestComponent {
	files := []TestFile{
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
	return TestComponent{
		Name:  "hello",
		Files: files,
		Output: []TestFile{
			{Path: "/etc/containers/systemd/hello/hello.container", Content: files[0].Content},
			{Path: "/var/lib/materia/components/hello/MANIFEST.toml"},
			{Path: "/var/lib/materia/components/hello/Containerfile", Content: files[2].Content},
			{Path: "/etc/containers/systemd/hello/hello.build", Content: files[3].Content},
		},
	}
}()

var instancedFiles = []TestFile{
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

var helloInstanced = TestComponent{
	Name:   "hello",
	Files:  instancedFiles,
	Output: instancedOutput,
}

var instancedOutput = []TestFile{
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
		Content: instancedFiles[1].Content,
	},
	{
		Path:    "/etc/containers/systemd/hello@foo/hello.volume",
		Content: instancedFiles[2].Content,
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
		Content: instancedFiles[1].Content,
	},
	{
		Path:    "/etc/containers/systemd/hello@bar/hello.volume",
		Content: instancedFiles[2].Content,
	},
}

var helloAll = TestComponent{
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
