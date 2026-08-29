---
title: MATERIA-SOURCE-OCI
section: 5
header: User Manual
footer: materia 0.7.1
date: August 2026
author: stryan
---

## Name
materia-source-oci - Configuration for Materia OCI Repository Source

## Synopsis

`/etc/materia/config.toml, $MATERIA_OCI__*`

## Description

Configures a remote OCI image as a the source for the Materia repository

The OCI image is expected to have the materia repository as its root file system


### Options

Note: the OCI source only works with remote images. You can not refer to a local image with this.

#### `MATERIA_OCI__USERNAME`/ **oci.username**

The username used to authenticate against the image repository.

#### `MATERIA_OCI__PASSWORD`/ **oci.password**

The password used to authenticate against the image repository.

#### `MATERIA_OCI__INSECURE`/ **oci.insecure**

Whether or not to allow insecure connections to the remote image repository.

#### `MATERIA_OCI__TAG`/ **oci.tag**

OCI image tag to use instead of what's in the source URL.
