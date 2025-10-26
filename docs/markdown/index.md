# Welcome to Materia

A GitOps style tool for managing services and applications deployed as [Quadlets](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html).

## Features

- Easy deployment: Grab the binary or pull the image, set the `MATERIA_SOURCE__URL` environment variable, and you're good to go
- Handles the full lifecycle of a service: no need to write tedious install and uninstall runbooks for each service you're running, just add or remove them from the manifest and Materia handles the rest.
- Share the work: Re-use other peoples Materia components with just a single line.
- Container-native and systemd-native management: works with the existing tools on your hosts instead of requiring you to install something else
- Minimal learning required: if you know how to write systemd unit files you're 90% of the way there
- Pull-based orchestration: Each node manages itself so no need for a controller or master node.

## Installation

Install a [release off Github](https://github.com/stryan/materia/releases/latest) or pull the container `podman pull ghcr.io/stryan/materia` onto the target node and you're set.

See the [Quickstart](quickstart.md) to get started.

## Requirements

The following are run time requirements and reflect what systems Materia is tested on. It may work without the specified versions (especially the systemd requirement).

Materia will not work with Podman versions lower than 4.4, as that is the version Quadlets were introduced in.

- Podman 5.4 or higher
- Systemd v254 or higher

Materia supports running both root-full and rootless quadlets, however currently root-full is the more tested pathway.

## Resources

[Reference Pages](./reference/index.md) For the most up-to-date and traditional man-style documentation
