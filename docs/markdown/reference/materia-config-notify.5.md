---
title: MATERIA-CONFIG-NOTIFY
section: 5
header: User Manual
footer: materia 0.7.1
date: August 2026
author: stryan
---

## Name
materia-config-notify - Materia notification settings

## Synopsis

`/etc/materia/config.toml`, `$MATERIA_NOTIFY__<option-name>`

## Description

Adjust how and where Materia sends notifications to.

## Options

#### *MATERIA_NOTIFY__TRIGGERS*/**notify.triggers**

What webhooks to use for what type of notification event. The following event types are supported:

- `default`: Default notification channel.
- `update`: Successful plan-execute cycles
- `rollback`: When a rollback is initiated

The values should be the webhook destination i.e.
~~~toml
[notify.triggers]
update = "https://localhost/webhook"
~~~
or `MATERIA_NOTIFY__TRIGGERS__UPDATE=https://localhost/webhook`
