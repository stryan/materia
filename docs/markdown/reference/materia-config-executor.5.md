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

#### *MATERIA_EXECUTOR_CLEANUP_COMPONENTS*/**cleanup_components**

Defaults to `false`.

If an error occurs while installing a component resulting in an execution failure, purge the failed component.
