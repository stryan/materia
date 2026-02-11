---
title: MATERIA-CONFIG-SERVICES
section: 5
header: User Manual
footer: materia 0.5.0
date: January 2026
author: stryan
---

## Name
materia-config-services - Materia services configuration settings

## Synopsis

`/etc/materia/config.toml`, `$MATERIA_SERVICES_<option-name>`

## Options

Presented in *environmental variable*/**TOML config line option** format.

#### *MATERIA_SERVICES__TIMEOUT*/**services.timeout**

Defaults to `90`.

How long to wait when starting/stopping systemd services when no service resource timeout is configured.

#### MATERIA_SERVICES__DRYRUN_QUADLETS/services.dryrun_quadlets

Defaults to false.

Whether to run a dry-run of the quadlet generator before starting/stopping services. Enable this if you need to make sure Quadlets are installed correctly before starting services.
