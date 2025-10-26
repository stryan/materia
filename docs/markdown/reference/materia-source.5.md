---
title: MATERIA-SOURCE
section: 5
header: User Manual
footer: materia 0.4.0
date: October 2025
author: stryan
---

## Name
materia-source - Configuration for Materia Repository Sources

## Synopsis

`/etc/materia/config.toml, $MATERIA_SOURCE__URL, $MATERIA_GIT__*, $MATERIA_FILE__*`

## Description

Materia needs to be able to clone its repository from a source. This is either a local directory or a remote Git repository.

## Options

Presented in *environmental variable*/**TOML config line option** format.

### Source Config

#### MATERIA_SOURCE__URL / source.url

Source location of the *materia-repository(5)* in URL format. Accepted formats:

    Git Repo: `git://git_repo_url`.

    Local file Repo: `file://<file_path>` e.g. `file:///tmp/materia_repo`

### Git Config

#### **MATERIA_GIT__BRANCH**/ **git.branch**

Git branch to checkout.

#### **MATERIA_GIT__PRIVATE_KEY**/ **git.private_key**

Private key used for SSH-based git operations

#### **MATERIA_GIT__USERNAME**, **MATERIA_GIT__PASSWORD**/ **git.username/git.password**

Username and password used for HTTP-based git operations

#### **MATERIA_GIT__KNOWNHOSTS**/ **git.knownhosts**

`knownhosts` file used for SSH-based git operations. Useful if you're running materia in a container.

#### **MATERIA_GIT__INSECURE**/ **git.insecure**

Disable SSH knownhosts checking for git SSH operations and use `http://` instead of `https://` for HTTP operations.
