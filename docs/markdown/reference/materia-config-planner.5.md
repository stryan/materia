---
title: MATERIA-CONFIG-PLANNER
section: 5
header: User Manual
footer: materia 0.5.0
date: December 2026
author: stryan
---

## Name
materia-config-planner - Materia planner configuration settings

## Synopsis

`/etc/materia/config.toml`, `$MATERIA_PLANNER_<option-name>`

## Options

Presented in *environmental variable*/**TOML config line option** format.

#### *MATERIA_PLANNER_CLEANUP_QUADLETS*/**cleanup_quadlets**

Removes non-volume Quadlets when their associated resources are removed. Defaults to false.

Example: If a resource `test.network` file is removed, materia will also run a `podman network rm systemd-test` command.

This is done on a best-effort basis. If a resource is in-use by other containers (whether materia managed or otherwise) materia will not attempt to remove it.

The following quadlet types are supported by this:

- Networks
- Images
- Build

#### *MATERIA_PLANNER_CLEANUP_VOLUMES*/**cleanup_volumes**

When removing a `.volume` Quadlet resource, remove the volume from Podman as well. Defaults to false.

This is separate from the above **cleanup_podman** option since volumes container user data. It is recommended to leave this to false or use this in conjunction with the **backupvolumes** option.

#### *MATERIA_PLANNER_BACKUP_VOLUMES*/**backup_volumes**

If an action would delete a Podman volume, create a backup of it first using `podman volume export` and store it in **output_dir**. Defaults to true.

Note, this only occurs if a Podman volume is actually being deleted e.g. `podman volume rm`. This does NOT create a backup if just the Quadlet file is deleted.

#### MATERIA_PLANNER_MIGRATE_VOLUMES/migrate_volumes

(EXPERIMENTAL)

Defaults to `false`.

If a volume quadlet is updated, instead of just updating the Quadlet file perform a data migration. A migration consists of the following steps:

    1. Stop services for the component
    2. Dump the existing volume to a tarball
    3. Delete the existing volume
    4. Update the quadlet
    5. Restart the updated service to create the new volume
    6. Import the old volume tarball into the new volume
