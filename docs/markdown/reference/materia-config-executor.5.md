---
title: MATERIA-CONFIG-EXECUTOR
section: 5
header: User Manual
footer: materia 0.6.0
date: February 2026
author: stryan
---

## Name
materia-config-executor - Materia executor configuration settings

## Synopsis

`/etc/materia/config.toml`, `$MATERIA_EXECUTOR_<option-name>`

## Options

Presented in *environmental variable*/**TOML config line option** format.

#### *MATERIA_EXECUTOR__CLEANUP_COMPONENTS*/**executor.cleanup_components**

Defaults to `false`.

If an error occurs while installing a component resulting in an execution failure, purge the failed component.

#### *MATERIA_EXECUTOR__MATERIA_DIR*/**executor.materia_dir**

Overrides the executor's configured materia data directory.

#### *MATERIA_EXECUTOR__QUADLET_DIR/**executor.quadlet_dir**

Overrides the executor's configured quadlet directory.

#### *MATERIA_EXECUTOR__SCRIPTS_DIR*/executor.scripts_dir*

Overrides the executor's configured scripts directory.

#### *MATERIA_EXECUTOR__SERVICE_DIR*/executor.service_dir*

Overrides the executor's configured service directory.

