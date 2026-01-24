# Materia

[![Chat on Matrix](https://matrix.to/img/matrix-badge.svg)](https://matrix.to/#/#materia:saintnet.tech)

A GitOps style tool for managing services and applications deployed as [Quadlets](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html).

Materia handles the full lifecycle of an application (or **component**):
1. It installs components and all their associated Quadlets and data files, templating files with variables and secrets if required
2. It starts services required by the component.
3. When updated files are found in the source repository it updates the installed versions and restarts services accordingly
4. And when a component is not longer assigned to a host, it stops all related services and removes the resources, keeping things nice and tidy.

See the [Documentation site](https://primamateria.systems) for more details and the [example repository](https://github.com/stryan/materia_example_repo) for what Materia repository looks like.

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

By default it is assumed you are running using root. If not, you'll need to update the bind mounts to their appropriate locations; see the [manual](./docs/markdown/reference/index.md) for more details. By default materia uses XDG_DIR settings in rootless mode.
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
	--env MATERIA_SOURCE__KIND="git" \
	--env MATERIA_SOURCE__URL=https://github.com/stryan/materia_example_repo \
	ghcr.io/stryan/materia:stable update
```

Note that some security settings may need to be adjusted based off your distro. For example, systems using AppArmor may require `PodmanArgs=--security-opt=apparmor=unconfined`.

See [install](./install/) for example Quadlets.

### Available tags

**stable**: Use the latest tagged release.

**v<tag>**: Specific tagged release.

**latest**: Latest push to master

# Quickstart

View the Quickstart guide on the [documentation site](https://primamateria.systems/quickstart.html).

# Contributing

 Questions or bug reports are welcome! Please start a Discussion versus opening an Issue, as Materia does bug tracking outside of Github using [git-bug](https://github.com/git-bug/git-bug). You can also submit bugs/suggestions or ask questions in the [Matrix room](https://matrix.to/#/#materia:saintnet.tech).

For submitting features/bugfixes/code-in general via merge requests, please see the [Contribution guide](CONTRIBUTING.md).
