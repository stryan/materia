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

#### MATERIA_SOURCE_KIND / source.kind

Remote source repository kind. Supported values: `git`,`file`.

If left empty materia will guess based off the provided URL. Otherwise the specified `source.url` will be provided directly to the source provider.

#### MATERIA_SOURCE__URL / source.url

Source location of the *materia-repository(5)* in URL format. Will be provided directly to the source provider.

If `source.kind` is not specified it will attempt to guess what source to use based off the following formats:

Accepted formats:

    Git Repo: `git://git_repo_url`. Will be treated as an HTTP(s) remote

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
