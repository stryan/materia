---
title: MATERIA-CONFIG-SERVER
section: 5
header: User Manual
footer: materia 0.6.0
date: February 2026
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

#### MATERIA_SERVER__NOTIFY_WEBHOOK/server.notify_webhook

Where to send webhook notifications on plan/update failure

#### MATERIA_SERVER__SYNC_WEBHOOK/server.sync_webhook

True/false. Whether to enable the HTTP `/webhook` listener. Accepts POST'ed JSON payloads in the following format:

```json
{
    "revision": "optional: revision to sync to",
    "update": true|false,
    "secret": "pre-shared secret: server.secret"
}
```

#### MATERIA_SERVER__SYNC_SECRET/server.sync_secret

Pre-shared secret for basic security on sync webhook

#### MATERIA_SERVER__SYNC_URL/server.sync_url

What URL the sync webhook listens on. Defaults to `:6284/webhook`

#### MATERIA_SERVER__SOCKET/server.socket

What Unix socket to listen on. Defaults to `/run/materia/materia.sock` for root and `/run/UID/materia/materia.sock` for rootless.

