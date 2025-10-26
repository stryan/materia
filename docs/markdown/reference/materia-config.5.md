---
title: MATERIA-CONFIG
section: 5
header: User Manual
footer: materia 0.3.0
date: October 2025
author: stryan
---

## Name
materia-config - Materia configuration settings

## Synopsis

`/etc/materia/config.toml`, `$MATERIA_<option-name>`

## Description

**materia** is designed to be entirely configured via environmental variables (`MATERIA_*`). However for administrative convenience it is possible to also configure it via a TOML config file, passed via the `-c` flag.

When both environmental variables and a config file are used, config file settings are overwritten by environmental variables.

Materia will by default use any and all configured attributes engines.

For configuring attributes management with `age`, see `materia-config-age(5)`.

For configuring attributes management with `sops`, see `materia-config-sops(5)`.

## Options

Presented in *environmental variable*/**TOML config line option** format.

#### MATERIA_ATTRIBUTES/attributes

Attributes Engine config to use. Optional, will cause materia to ignore any other configured engines.

#### *MATERIA_HOSTNAME*/**hostname**

Hostname to use for fact generation and component assignment. If not specified, defaults to system hostname

#### *MATERIA_DEBUG*/**debug**

Enable extra debug logging. Default false

#### *MATERIA_USE_STDOUT*/**use_stdout**

Log to `STDOUT` instead of `STDERR`

#### *MATERIA_ROLES*/**roles**

Use these assigned roles instead of what's in the `materia-manifest(5)`

#### *MATERIA_DIFFS*/**diffs**

When calculating resource differences, show diffs. Default false.

#### *MATERIA_TIMEOUT*/**timeout**

How long to wait when starting/stopping systemd services. Default 30 seconds.

#### *MATERIA_NO_SYNC*/**no_sync**

Do not sync source repository before running operations.

#### *MATERIA_CLEANUP*/**cleanup**

If an error occurs while installing a component, don't leave any files behind. Defaults false.

#### *MATERIA_MATERIA_DIR*/**materia_dir**

Root directory for materia directories. Defaults to `/var/lib/materia` for root and `XDG_DATA_HOME/.local/share/materia` for nonroot.

#### *MATERIA_SOURCE_DIR*/**source_dir**

Directory where materia keeps local cache of source repository. Defaults to `PREFIX/source`

#### *MATERIA_OUTPUT_DIR*/**output_dir**

Directory where materia outputs `lastrun` and `plan` files. Defaults to `PREFIX/output`

#### *MATERIA_QUADLET_DIR*/**quadlet_dir**

Directory where materia installs quadlet files. Defaults to `/etc/containers/systemd` for root and `XDG_CONFIG_HOME/containers/systemd` for nonroot.

#### *MATERIA_SERVICE_DIR*/**service_dir**

Directory where materia installs non-generated systemd unit files. Defaults to `/etc/systemd/system` for root and `XDG_DATA_HOME/systemd/user` for nonroot.

#### *MATERIA_SCRIPTS_DIR*/**scripts_dir**

Directory where materia installs scripts resources. Defaults to `/usr/local/bin/` for root and `$HOME /.local/bin` for nonroot.

#### *MATERIA_CLEANUP*/**cleanup**

When removing Quadlet resources that aren't volumes, remove the resources from Podman as well. Defaults to false.

Example: If a resource `test.network` file is removed, materia will also run a `podman network rm systemd-test` command.

#### *MATERIA_CLEANUP_VOLUMES*/**cleanup_volumes**

When removing a `.volume` Quadlet resource, remove the volume from Podman as well. Defaults to false.

This is separate from the above **cleanup** option since volumes container user data. It is recommended to leave this to false or use this in conjunction with the **backupvolumes** option.

#### *MATERIA_BACKUP_VOLUMES*/**backup_volumes**

If an action would delete a Podman volume, create a backup of it first using `podman volume export` and store it in **output_dir**. Defaults to true

Note, this only occurs if a Podman volume is actually being deleted e.g. `podman volume rm`. This does NOT create a backup if just the Quadlet file is deleted.

#### MATERIA_MIGRATE_VOLUMES/migrate_volumes

(EXPERIMENTAL)

If a volume quadlet is updated, instead of just updating the Quadlet file perform a data migration. A migration consists of the following steps:

    1. Stop services for the component
    2. Dump the existing volume to a tarball
    3. Delete the existing volume
    4. Update the quadlet
    5. Restart the updated service to create the new volume
    6. Import the old volume tarball into the new volume

#### MATERIA_SECRETS_PREFIX/secrets_prefix

Sets the prefix Materia appends to Podman secrets it manages. Defaults to `materia-`

#### MATERIA_SERVER__UPDATE_INTERVAL/server.update_interval

How long (in seconds) for `materia server` to wait before running a `materia update`.

#### MATERIA_SERVER__PLAN_INTERVAL/server.plan_interval

How long (in seconds) for `materia server` to wait before running a `materia plan`.

#### MATERIA_SERVER__WEBHOOK/server.webhook

Where to send webhook notifications on plan/update failure

#### MATERIA_SERVER__SOCKET/server.socket

What Unix socket to listen on. Defaults to `/run/materia/materia.sock` for root and `/run/UID/materia/materia.sock` for rootless.
