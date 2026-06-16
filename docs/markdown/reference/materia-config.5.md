---
title: MATERIA-CONFIG
section: 5
header: User Manual
footer: materia 0.7.0
date: June 2026
author: stryan
---

## Name
materia-config - Materia configuration settings

## Synopsis

`/etc/materia/config.toml`, `$MATERIA_<option-name>`

## Description

Materia is designed to be entirely configured via environmental variables (`MATERIA_*`). However for administrative convenience it is possible to also configure it via a TOML config file.

Materia will automatically attempt to use the config file located at `/etc/materia/config.toml`. Alternatively, `-c` can be used to specify the file.

When both environmental variables and a config file are used, config file settings are overwritten by environmental variables.

Materia will by default use any and all configured attributes engines.

For configuring extra planner features (Podman resource cleanup, volume data migration, etc) see `materia-config-planner(5)`.

For configuring extra execution features (Remove invalid components on failed execution, etc) see `materia-config-executor(5)`.

For configuring server mode features, see `materia-config-server(5)`.

For configuring attributes management with `age`, see `materia-config-age(5)`.

For configuring attributes management with `sops`, see `materia-config-sops(5)`.

## Options
Presented in *environmental variable*/**TOML config line option** format.

#### *MATERIA_ATTRIBUTES*/**attributes**

Attributes Engine config to use. Optional, if not configured Materia will use all attributes engines configured.

If set, Materia will ignore all configured attributes engines besides the one specified.

Ensures there is a default configuration for the engine.

#### *MATERIA_HOSTNAME*/**hostname**

Hostname to use for fact generation and component assignment. If not specified, defaults to system hostname

#### *MATERIA_DEBUG*/**debug**

Enable extra debug logging. Default false

#### *MATERIA_USE_STDOUT*/**use_stdout**

Log to `STDOUT` instead of `STDERR`

#### *MATERIA_ROLES*/**roles**

Use these assigned roles instead of what's in the `materia-manifest(5)`

#### *MATERIA_NO_SYNC*/**no_sync**

Do not sync source repository before running operations.

#### *MATERIA_MATERIA_DIR*/**materia_dir**

Root directory for materia directories. Defaults to `/var/lib/materia` for root and `XDG_DATA_HOME/materia` for nonroot.

#### *MATERIA_SOURCE_DIR*/**source_dir**

Directory where materia keeps local cache of source repository. Defaults to `MATERIA_DATA_DIR/source`

#### *MATERIA_OUTPUT_DIR*/**output_dir**

Directory where materia outputs `lastrun` and `plan` files. Defaults to `MATERIA_DATA_DIR/output`

#### *MATERIA_QUADLET_DIR*/**quadlet_dir**

Directory where materia installs quadlet files. Defaults to `/etc/containers/systemd` for root and `XDG_CONFIG_HOME/containers/systemd` for nonroot.

#### *MATERIA_SERVICE_DIR*/**service_dir**

Directory where materia installs non-generated systemd unit files. Defaults to `/etc/systemd/system` for root and `XDG_DATA_HOME/systemd/user` for nonroot.

#### *MATERIA_SCRIPTS_DIR*/**scripts_dir**

Directory where materia installs scripts resources. Defaults to `/usr/local/bin/` for root and `$HOME /.local/bin` for nonroot.

#### *MATERIA_ROOTLESS*/**materia.rootless**

(EXPERIMENTAL)

Enables `rootless` mode for Materia in a container. Causes materia to parse its own container's bind mounts to determine where on the host machine directories are. Use when you're running materia in a rootless container and are bind-mounting the user directories to the normal materia root directories in the container i.e. `-v /home/user/.config/containers/systemd:/etc/containers/systemd`.

#### *MATERIA_APPMODE*/**appmode**

Generate `.app` files with when installing quadlets to keep them compatibile with Podman 5 `podman quadlet commands`.

Soft-Deprecated: Podman 6 no longer uses `.app` files so this feature will not be updated

#### *MATERIA_LOCK*/**lock**

(EXPERIMENTAL)

Enable locking to prevent multiple Materia or Materia related processes from interfering with each other.

Valid options are `dbus` or `file`. Dbus based locking may require a dbus policy to be installed.

#### *MATERIA_ROLLBACK*/**rollback**

(EXPERIMENTAL)

Enables `rollback` mode for the `update` command. When this is enabled, Materia will detect failures during an update and, if possible, rollback to a previous state of the source repository and re-run the update with whatever rollback method that repository source supports.

This feature currently only works with the `git` source. When rolling back, Materia will checkout whatever commit the local repository cache was on before the most recent sync.

The setting value will determine what health check system to use. Currently the only supported option is "service": Materia will rollback if a service state change causes the service to enter the `failed` state or if the final service check reports a different state than expected.

Valid options: "service".


#### *MATERIA_PODMAN_COMMAND*/**podman_command**

Fallback to using the old system of wrapping the `podman` command for container operations. Use if you're seeing errors accessing podman containers/secrets/volumes/etc.

Will be removed in 0.8.
