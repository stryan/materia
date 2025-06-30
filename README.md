# Materia
A GitOps style tool for managing services and applications deployed as [Quadlets](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html).

Materia handles the full lifecycle of an application (or **component**):
1. It installs components and all their associated Quadlets, templating files with variables and secrets if required
2. It starts services required by the component
3. When updated files are found in the source repository it updates the installed versions and restarts services accordingly
4. And when a component is not longer assigned to a host, it stops all related services and removes the resources, keeping things nice and tidy.

# Install

## From source
Build from source using `mise build`. By default this will generate binaries for amd64 and arm64.

If you'd like to build without mise, you can do so through the normal go methods such as: `go build -ldflags="-w -s" -o bin/materia-arm64 ./cmd/materia/`

## With Podman

For obvious reasons, materia can only be run using `podman` as your container engine.

By default it is assumed you are running using root. If not, you'll need to update the bind mounts to their appropriate locations; see the [manual](./docs/index.md) for more details. By default materia uses XDG_DIR settings.
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
	-v /etc/materia/_key.txt:/etc/materia/key.txt \ #Optional, used for age decryption
	-v /etc/materia/materia_key:/etc/materia/materia_key \ # Optional, used for git+ssh checkouts
	--env MATERIA_AGE_KEYFILE=/etc/materia/key.txt \
	--env MATERIA_GIT_PRIVATEKEY=/etc/materia/materia_key \
	--env MATERIA_SOURCE_URL=git://git@github.commateria/materia_repo \
	ghcr.io/stryan/materia:latest
```

Note that some security settings may need to be adjusted based off your distro. For example, systems using AppArmor may require `PodmanArgs=--security-opt=apparmor=unconfined`.

See [install](./install/) for an example Quadlet.

# Quickstart

## Terminology

**target**: The node materia is running on

**repository**: A local directory or Git repository containing Materia components,manifests, and resources.

**manifest**: A TOML file containing metadata. Found at the **repository** level and the **component** level.

**component**: A collection of resources. Similar to an ansible role and is the basic building block of a repository.

**resource**: An individual file that is installed as part of a component and removed when the component is removed. Includes quadlets, data files, non-generated systemd units, scripts, and more.

## Install materia on the destination node

Follow the instructions under Install and get that binary or container on the target.

For this quickstart we will assume you're using the raw binary on machine "testhost". We will also assume that A) you are running the binary as root and B) the root user is already set up for password-less SSH to your Git forge of choice.

## Setup your repository

For a more in-depth look at setting up a repository, see the [example repo](docs/example_repo) and the [repository documentation](/docs/materia-repository.5.md).

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
[defaults]
containerTag = "latest"
[[services]]
service = "hello.service"
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
Materia is designed to be configured with environment variables; if you would like to use config files see the [config docs](docs/materia-config.5.md).

Since we're not using any [secrets](docs/materia-config-age.5.md) we only need to set the source URL:

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

Assuming the plan was generated succesfully, you can now run the actual update:

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

# FAQ

## Why should I use this over Ansible/Puppet/Chef/some other configuration management?

Short answer: Because writing complicated YAML playbooks and Ruby modules is a pain and it's way easy to just drop a few unit files in a Git repo.

Longer answer: This is an opinionated tool with a tighter field of concern (applications/servers managed by Quadlets instead of full system state) it can afford to do much more without user intervention.
For example, when the services list for a component changes Materia will automatically stop the old services without the user having to specify.
It also supports tighter integrations with container native features like volume backups and (in the near future) recreating volumes and auto-migrating data.

## Why should I use this over Kubernetes?

Because you don't have a cluster and/or you probably don't need one.

Materia is designed for smaller-scale deployments. It is not intended to replace the full scale of Kubernetes or any similar software.

At some point I'd like to look into adding BlueChi integration so that components can have dependencies on other nodes, but that's far in the future.

## Can I manage my whole system with this?

Probably not. Materia is specifically designed to handle the application level; it does not and will not do stuff like my create or remove users, add firewall rules, etc. I highly encourage using *materia* along with something like Ansible or Terraform to handle the rest of the systems needs.

The rough intention is that this tool is deployed on top of an atomic distro like **OpenSUSE MicroOS** or **Fedora CoreOS**, where the majority of the system is controlled as a read-only image and the application servers (like app backends and `nginx`) are run as containers in the read/write section.
While traditional distros are a supported platform, most design work and testing is done assuming one of the above.

## Will more secrets backends be supported besides age?

Yes. I hope to add other secrets backends like Vault soon; `age` was just an easy first target and works well with the GitOps style.

On a related note, secrets management tools like `ansible-vault` are not within scope of the project. It is expected to manage secrets entirely with a third party tool.

## Is this related to Final Fantasy?

No, it's named after the alchemical concept of the [prima materia](https://en.wikipedia.org/wiki/Prima_materia). All of the materia-related tools are named after alchemical terms.

Also the last good Final Fantasy game was FF3 and I will die on this hill.

## Is this related to \<insert any of the several companies named materia\>

No, but it's funny we all get out-SEO'd by a video game.

