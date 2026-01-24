---
title: MATERIA-SOURCE
section: 5
header: User Manual
footer: materia 0.5.0
date: January 2026
author: stryan
---

## Name
materia-source - Configuration for Materia Repository Sources

## Synopsis

`/etc/materia/config.toml, $MATERIA_SOURCE__URL, $MATERIA_GIT__*, $MATERIA_FILE__*`

## Description

Materia needs to be able to clone its repository from a source. This is either a local directory, a remote Git repository, or a remote OCI image.

## Options

Presented in *environmental variable*/**TOML config line option** format.

### Source Config

#### MATERIA_SOURCE_KIND / source.kind

Remote source repository kind. Supported values: `git`,`file`,`oci`.

If left empty materia will guess based off the provided URL. Otherwise the specified `source.url` will be provided directly to the source provider.

#### MATERIA_SOURCE__URL / source.url

Source location of the *materia-repository(5)* in URL format. Will be provided directly to the source provider.

(The following behaviour is deprecated and will be removed in v0.7)

If `source.kind` is not specified it will attempt to guess what source to use based off the following formats:

Accepted formats:

    Git Repo: `git://git_repo_url`. Will be treated as an HTTP(s) remote

    Local file Repo: `file://<file_path>` e.g. `file:///tmp/materia_repo`

### Git Config

#### **MATERIA_GIT__BRANCH**/ **git.branch**

Git branch to checkout.

#### MATERIA_GIT__DEFAULT/ git.default

The Git branch to checkout if `git.branch` isn't specified. Defaults to `master`.

#### **MATERIA_GIT__PRIVATE_KEY**/ **git.private_key**

Private key used for SSH-based git operations

#### **MATERIA_GIT__USERNAME**, **MATERIA_GIT__PASSWORD**/ **git.username/git.password**

Username and password used for HTTP-based git operations

#### **MATERIA_GIT__KNOWNHOSTS**/ **git.knownhosts**

`knownhosts` file used for SSH-based git operations. Useful if you're running materia in a container.

#### **MATERIA_GIT__INSECURE**/ **git.insecure**

Disable SSH knownhosts checking for git SSH operations and use `http://` instead of `https://` for HTTP operations.

#### MATERIA_GIT__CAREFUL/ git.careful

Prevents materia from running git operations that would overwrite git history (i.e. anything requiring `--force`). Defaults to `false`.

### OCI Config

Note: the OCI source only works with remote images. You can not refer to a local image with this.

#### MATERIA_OCI__USERNAME/ oci.username

The username used to authenticate against the image repository.

#### MATERIA_OCI__PASSWORD/ oci.password

The password used to authenticate against the image repository.

#### MATERIA_OCI__INSECURE/ oci.insecure

Whether or not to allow insecure connections to the remote image repository.

#### MATERIA_OCI__TAG/ oci.tag

OCI image tag to use instead of what's in the source URL.
