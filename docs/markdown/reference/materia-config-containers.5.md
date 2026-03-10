---
title: MATERIA-CONFIG-CONTAINERS
section: 5
header: User Manual
footer: materia 0.6.2
date: March 2026
author: stryan
---

## Name
materia-config-containers - Materia containers configuration settings

## Synopsis

`/etc/materia/config.toml`, `$MATERIA_CONTAINERS_<option-name>`

## Options

Presented in *environmental variable*/**TOML config line option** format.

#### *MATERIA_CONTAINERS__REMOTE*/**containers.remote**

Whether materia is controlling a remote podman instance or not. Defaults to `true` when run in a container. Don't mess with this unless you know what you're doing.


#### *MATERIA_CONTAINERS__SECRETS_PREFIX*/**containers.secrets_prefix**

Sets the prefix Materia appends to Podman secrets it manages. Defaults to `materia-`


#### *MATERIA_CONTAINERS__COMPRESSION_COMMAND/**containers.compression_command**

When performing a volume backup (either through a manually triggered dump action or volume migration or otherwise), pipe the output of the export through this command.

If no command is provided, Materia will output volumes in the standard `tar` format, equivalent to performing a `podman volume export`.

#### *MATERIA_CONTAINERS__COMPRESSION_SUFFIX/**containers.compression_suffix**

What file type to append to compressed volume backups. Defaults to `.gz` for `gzip`, `.zstd` for `zstd`,`.zip` for `zip`, and `.compressed` when otherwise not set.
