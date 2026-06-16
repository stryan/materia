---
title: MATERIA-CONFIG-CONTAINERS
section: 5
header: User Manual
footer: materia 0.7.0
date: March 2026
author: stryan
---

## Name
materia-config-containers - Materia containers configuration settings

## Synopsis

`/etc/materia/config.toml`, `$MATERIA_CONTAINERS__<option-name>`

## Options

Presented in *environmental variable*/**TOML config line option** format.

#### *MATERIA_CONTAINERS__REMOTE*/**containers.remote**

Whether materia is controlling a remote podman instance or not. Defaults to `true` when run in a container. Don't mess with this unless you know what you're doing.

This option only works when `MATERIA_PODMAN_COMMAND` is set and will be deprecated at the same time as that flag.

#### *MATERIA_CONTAINERS__SECRETS_PREFIX*/**containers.secrets_prefix**

Sets the prefix Materia appends to Podman secrets it manages. Defaults to `materia-`


#### *MATERIA_CONTAINERS__COMPRESSION_COMMAND/**containers.compression_command**

If set, volumes created by a dump action or volume migration will be compressed with this format.

Valid options: "gzip" or "zstd".

Volume dump file are now in the format `volumename-volume.tar(.gz/zst)`.
