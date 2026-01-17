---
title: MATERIA-CONFIG-EXECUTOR
section: 5
header: User Manual
footer: materia 0.5.0
date: January 2026
author: stryan
---

## Name
materia-config-executor - Materia executor configuration settings

## Synopsis

`/etc/materia/config.toml`, `$MATERIA_PLANNER_<option-name>`

## Options

Presented in *environmental variable*/**TOML config line option** format.

#### *MATERIA_EXECUTOR__CLEANUP_COMPONENTS*/**cleanup_components**

Defaults to `false`.

If an error occurs while installing a component resulting in an execution failure, purge the failed component.

#### MATERIA_EXECUTOR__MATERIA_DIR/materia_dir

Overrides the executor's configured materia data directory.

#### MATERIA_EXECUTOR__QUADLET_DIR/quadlet_dir

Overrides the executor's configured quadlet directory.

#### MATERIA_EXECUTOR__SCRIPTS_DIR/scripts_dir

Overrides the executor's configured scripts directory.

#### MATERIA_EXECUTOR__SERVICE_DIR/service_dir

Overrides the executor's configured service directory.

