# Materia
A GitOps style tool for managing services and applications deployed as [Quadlets](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html).

Materia handles the full lifecycle of an application (or **component**):
1. It installs components and all their associated Quadlets, templating files with variables and secrets if required
2. It starts services required by the component
3. When updated files are found in the source repository it updates the installed versions and restarts services accordingly
4. And when a component is not longer assigned to a host, it stops all related services and removes the resources, keeping things nice and tidy.

See the [Documentation site](https://primamateria.systems) for more details, or read on for a quick start.

# Install

## Requirements

The following are run time requirements and reflect what systems Materia is tested on. It may work without the specified versions (especially the systemd requirement).

Materia will not work with Podman versions lower than 4.4, as that is the version Quadlets were introduced in.

- Podman 5.4 or higher
- Systemd v254 or higher

Materia supports running both root-full and rootless quadlets, however currently root-full is the more tested pathway.

## From source
Build from source using `mise build`. By default this will generate binaries for amd64 and arm64.

If you'd like to build without mise, you can do so through the normal go methods such as: `go build -ldflags="-w -s" -o bin/materia-arm64 ./cmd/materia/`

## From Binary

Grab a release for your architecture from the releases page; the static binaries should work on any relatively recent Linux distro.

## With Podman

For obvious reasons, materia should only be run using `podman` as your container engine.

By default it is assumed you are running using root. If not, you'll need to update the bind mounts to their appropriate locations; see the [manual](docs/markdown/index.md) for more details. By default materia uses XDG_DIR settings.
```
podman run --name materia --rm \
	--hostname <system_hostname> \
	--network host \
	--security-opt label=disable \ # optional, depending on OS security settings
	-v /run/dbus/system_bus_socket:/run/dbus/system_bus_socket \ # needed to manage systemd units
	-v /run/podman/podman.sock:/run/podman/podman.sock \ # needed to get container status
	-v /var/lib/materia:/var/lib/materia \ # Where materia stores its source cache and component data
	-v /etc/containers/systemd:/etc/containers/systemd \ # needed to install Quadlets
	-v /usr/local/bin:/usr/local/bin \ # customizable, change to where ever you want scripts to be installed to
	-v /etc/systemd/system:/etc/systemd/system \ # Needed to manage services, can also use /usr/local/lib/systemd/system/
	-v /etc/materia/known_hosts:/root/.ssh/known_hosts:ro \ #Optional, used for git+ssh checkouts
	-v /etc/materia/key.txt:/etc/materia/key.txt \ #Optional, used for age decryption
	-v /etc/materia/materia_key:/etc/materia/materia_key \ # Optional, used for git+ssh checkouts
	--env MATERIA_AGE__KEYFILE=/etc/materia/key.txt \
	--env MATERIA_GIT__PRIVATE_KEY=/etc/materia/materia_key \
	--env MATERIA_SOURCE__URL=git://git@github.com/stryan/materia_example_repo \
	ghcr.io/stryan/materia:stable update
```

Note that some security settings may need to be adjusted based off your distro. For example, systems using AppArmor may require `PodmanArgs=--security-opt=apparmor=unconfined`.

See [install](./install/) for an example Quadlet.

### Available tags

**stable**: Use the latest tagged release

**v<tag>**: Specify tagged release

**latest**: Latest push to master

# Quickstart

## Install materia on the destination node

Follow the instructions under Install and get that binary or container on the target.

For this quickstart we will assume you're using the raw binary on machine "testhost". We will also assume that A) you are running the binary as root and B) the root user is already set up for password-less SSH to your Git forge of choice.

## Setup your repository

For a more in-depth look at setting up a repository, see the [example repo](https://github.com/stryan/materia_example_repository) and the [repository documentation](docs/markdown/reference/materia-repository.5.md).

On your workstation, create a bare Git repository with the following directories:

```
repo/
repo/components
repo/components/hello
```

### Create a component
Create the following *quadlet resource* in the hello *component* directory:
```
cat > repo/components/hello/hello.container.gotmpl << EOL
[Unit]
Description=Hello Service

[Container]
ContainerName=busybox1
Image=docker.io/busybox:{{.containerTag}}
Exec=/bin/sh -c "trap 'exit 0' INT TERM; while true; do echo Hello World; sleep 1; done"

[Install]
WantedBy=multi-user.target
EOL
```

A resource can be any file type; resource files ending with `.gotmpl` are interpreted as Go Templates.

Create the following *manifest resource* for the component:
```
cat > repo/components/hello/MANIFEST.toml << EOL
[Defaults]
containerTag = "latest"
[[Services]]
Service = "hello.service"
EOL
```

Manifest resources always have the file name `MANIFEST.toml`

### Assign components to a localhost

Create the following *repository manifest* in the top level of the repository:
```
cat > repo/MANIFEST.toml << EOL
[hosts.testhost]
components = ["hello"]
EOL
```

### Push the repository to your forge of choice

```
git remote add origin git@github.com:user/materia_repo
git push
```

## Run a test plan

### Set environment variables
Materia is designed to be configured with environment variables; if you would like to use config files see the [config docs](docs/markdown/reference/materia-config.5.md).

Since we're not using any [attributes](docs/markdown/attributes.md) we only need to set the source URL:

`export MATERIA_SOURCE_URL="git://git@github.com:user/materia_repo"`

### Generate the test plan

Assuming the `materia` binary is on your path, run `materia plan`:
```
$ materia plan
Plan:
1. Installing component hello
2. Templating container resource hello/hello.container
3. Reloading systemd units
4. Starting service hello/hello.service

$
```

## Run an update

Assuming the plan was generated successfully, you can now run the actual update:

```
$ materia update
Plan:
1. Installing component hello
2. Templating container resource hello/hello.container
3. Reloading systemd units
4. Starting service hello/hello.service

$ ls /etc/containers/systemd/
hello
$ ls -a /etc/containers/systemd/hello
. .. hello.container .materia_managed
$ ls -a /var/lib/materia/components/hello
. .. .component_version
$ systemctl is-active hello.service
active
$
```

# Contributing

If you have any questions or issues, please start a Discussion versus opening an Issue, as Materia does bug tracking outside of Github using [git-bug](https://github.com/git-bug/git-bug).
