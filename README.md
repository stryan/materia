# Materia
A GitOps style tool for managing Podman Quadlet files.

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
	--env MATERIA_AGE_IDENTS=/etc/materia/key.txt \
	--env MATERIA_GIT_PRIVATEKEY=/etc/materia/materia_key \
	--env MATERIA_SOURCEURL=git://git@git.saintnet.tech:stryan/saintnet_materia \
	git.saintnet.tech/stryan/materia:latest
```

Note that some security settings may need to be adjusted based off your distro. For example, systems using AppArmor may require `PodmanArgs=--security-opt=apparmor=unconfined`.

See [install](./install/) for an example Quadlet.

# Quickstart

## Setup your repository

## Run a test plan

## Run an update

# FAQ
