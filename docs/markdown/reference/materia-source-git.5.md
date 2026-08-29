---
title: MATERIA-SOURCE-GIT
section: 5
header: User Manual
footer: materia 0.7.1
date: August 2026
author: stryan
---

## Name
materia-source-git - Configuration for Materia Git Repository Source

## Synopsis

`/etc/materia/config.toml, $MATERIA_GIT__*`

## Description

Configures a remote Git repositroy as the source for the Materia repository


### Options

#### `MATERIA_GIT__BRANCH`/ **git.branch**

Git branch to checkout.

#### `MATERIA_GIT__DEFAULT`/ **git.default**

(SOFT DEPRECATED)

The Git branch to checkout if `git.branch` isn't specified. Defaults to `master`.

There is nothing this config option does that **git.branch** can't do as well. Kept it for legacy configs and future updates.

#### `MATERIA_GIT__PRIVATE_KEY`/ **git.private_key**

Private key used for SSH-based git operations

#### `MATERIA_GIT__USERNAME`, `MATERIA_GIT__PASSWORD`/ **git.username/git.password**

Username and password used for HTTP-based git operations

#### `MATERIA_GIT__KNOWNHOSTS`/ **git.knownhosts**

`knownhosts` file used for SSH-based git operations. Useful if you're running materia in a container.

#### `MATERIA_GIT__INSECURE`/ **git.insecure**

Disable SSH knownhosts checking for git SSH operations and use `http://` instead of `https://` for HTTP operations.

#### `MATERIA_GIT__CAREFUL`/ **git.careful**

Prevents materia from running git operations that would overwrite git history (i.e. anything requiring `--force`). Defaults to `false`.
