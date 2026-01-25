---
title: MATERIA-CONFIG-SERVER
section: 5
header: User Manual
footer: materia 0.5.0
date: January 2026
author: stryan
---

## Name
materia-config-server - Materia server mode configuration settings

## Synopsis

`/etc/materia/config.toml`, `$MATERIA_SERVER__<option-name>`

## Options

#### MATERIA_SERVER__UPDATE_INTERVAL/server.update_interval

How long (in seconds) for `materia server` to wait before running a `materia update`.

#### MATERIA_SERVER__PLAN_INTERVAL/server.plan_interval

How long (in seconds) for `materia server` to wait before running a `materia plan`.

#### MATERIA_SERVER__WEBHOOK/server.webhook

Where to send webhook notifications on plan/update failure

#### MATERIA_SERVER__SOCKET/server.socket

What Unix socket to listen on. Defaults to `/run/materia/materia.sock` for root and `/run/UID/materia/materia.sock` for rootless.

