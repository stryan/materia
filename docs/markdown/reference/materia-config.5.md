---
title: MATERIA-CONFIG
section: 5
header: User Manual
footer: materia 0.1.0
date: June 2025
author: stryan
---

# Name
materia-config - Materia configuration settings

# Synopsis

`/etc/materia/config.toml`, `$MATERIA_<option-name>`

# Description

**materia** is designed to be entirely configured via environmental variables (`MATERIA_*`). However for administrative convenience it is possible to also configure it via a TOML config file, passed via the `-c` flag.

When both environmental variables and a config file are used, config file settings are overwritten by environmental variables.

For configuring secrets management with `age`, see `materia-config-age(5)`.

# Options

Presented in *environmental variable*/`TOML config line option` format.

*MATERIA_SOURCE_URL*/`sourceurl`

:  Source location of the *materia-repository(5)* in URL format. Accepted formats:

   Git Repo: `git://git_repo_url`. See *materia-config-git(5)* for more details.

   Local file Repo: `file://<file_path>` e.g. `file:///tmp/materia_repo`

*MATERIA_HOSTNAME*/`hostname`

:  Hostname to use for fact generation and component assignment. If not specified, defaults to system hostname

*MATERIA_DEBUG*/`debug`

:  Enable extra debug logging. Default false

*MATERIA_USESTDOUT*/`stdout`

:  Log to `STDOUT` instead of `STDERR`

*MATERIA_ROLES*/`roles`

:  Use these assigned roles instead of what's in the `materia-manifest(5)`

*MATERIA_DIFFS*/`diffs`

:  When calculating resource differences, show diffs. Default false.

*MATERIA_TIMEOUT*/`timeout`

:  How long to wait when starting/stopping systemd services. Default 30 seconds.

*MATERIA_NOSYNC*/`nosync`

:  Do not sync source repository before running operations.

*MATERIA_CLEANUP*/`cleanup`

:  If an error occurs while installing a component, don't leave any files behind. Defaults false.

*MATERIA_PREFIX*/`prefix`

:  Root directory for materia directories. Defaults to `/var/lib/materia` for root and `XDG_DATA_HOME/.local/share/materia` for nonroot.

*MATERIA_SOURCEDIR*/`sourcedir`

:  Directory where materia keeps local cache of source repository. Defaults to `PREFIX/source`

*MATERIA_OUTPUTDIR*/`outputdir`

:  Directory where materia outputs `lastrun` and `plan` files. Defaults to `PREFIX/output`

*MATERIA_QUADLETDIR*/`quadletdir`

:  Directory where materia installs quadlet files. Defaults to `/etc/containers/systemd` for root and `XDG_CONFIG_HOME/containers/systemd` for nonroot.

*MATERIA_SERVICEDIR*/`servicedir`

:  Directory where materia installs non-generated systemd unit files. Defaults to `/etc/systemd/system` for root and `XDG_DATA_HOME/systemd/user` for nonroot.

*MATERIA_SCRIPTSDIR*/`scriptsdir`

:  Directory where materia installs scripts resources. Defaults to `/usr/local/bin/` for root and `$HOME /.local/bin` for nonroot.
