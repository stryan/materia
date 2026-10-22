---
title: MATERIA-CONFIG-SERVER
section: 5
header: User Manual
footer: materia 0.8.0
date: October 2026
author: stryan
---

## Name
materia-config-server - Materia server mode configuration settings

## Synopsis

`/etc/materia/config.toml`, `$MATERIA_SERVER__<option-name>`

## Description

Settings that only apply to Materia when run in server mode (with `materia server`).

## Options

#### `MATERIA_SERVER__UPDATE_INTERVAL`/**server.update_interval**

How long (in seconds) for `materia server` to wait before running a `materia update`.

#### `MATERIA_SERVER__PLAN_INTERVAL`/**server.plan_interval**

How long (in seconds) for `materia server` to wait before running a `materia plan`.

### `MATERIA_SERVER__HEALTH`/`server.health`

Whether to enable the `/health` endpoint. Defaults to `true` if either `server.update_webhook` or `server.facts_endpoint` is true.

#### `MATERIA_SERVER__NOTIFY_WEBHOOK`/**server.notify_webhook**

Where to send webhook notifications on plan/update failure.

**DEPRECATED**: This has been replaced by the materia-config-notify.5 config.

#### `MATERIA_SERVER__UPDATE_WEBHOOK`/**server.update_webhook**

True/false. Whether to enable the HTTP `/webhook` listener. Accepts POST'ed JSON payloads in the following format:

~~~json
{
    "revision": "optional: revision to sync to",
    "update": true|false,
    "secret": "pre-shared secret: server.secret"
}
~~~

Note: `secret` is deprecated and will be replaced with `server.token`

#### `MATERIA_SERVER__FACTS_ENDPOINT`/**server.facts_endpoint**

True/false. Whether to enable the HTTP `/facts` listener. Accepts GETs with `Bearer token` authentication and returns all facts current known as JSON.

### `MATERIA_SERVER__TOKEN`/**server.token**

Pre-shared secret to use as a `Bearer token` header.

#### `MATERIA_SERVER__UPDATE_SECRET`/**server.update_secret**

Pre-shared secret for basic security on update webhook

**DEPRECATED**: Use `server.token` instead

#### `MATERIA_SERVER__URL`/**server.url**

What URL the webhooks listen on. Defaults to `:6284/webhook|facts`

#### `MATERIA_SERVER__UPDATE_URL`/**server.update_url**

What URL the update webhook listens on. Defaults to `:6284/webhook`

**DEPRECATED**: use `server.url` instead

#### `MATERIA_SERVER__SOCKET`/**server.socket**

What Unix socket to listen on for the Varlink API.

Defaults to `unix:/run/materia/materia.sock` for root and `unix:/run/UID/materia/materia.sock` for rootless.

#### `MATERIA_SERVER__QUIT_ON_ERROR`/**server.quit_on_error**

Whether the server should quit on error. Defaults to false.
