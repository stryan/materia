---
title: MATERIA-SOURCE
section: 5
header: User Manual
footer: materia 0.7.1
date: August 2026
author: stryan
---

## Name
materia-source - Configuration for Materia Repository Sources

## Synopsis

`/etc/materia/config.toml, $MATERIA_SOURCE__*`

## Description

Materia needs to be able to clone its repository from a source. This is either a local directory, a remote Git repository, or a remote OCI image.


## Options

Presented in *environmental variable*/**TOML config line option** format.

#### *MATERIA_SOURCE__KIND* / **source.kind**

Remote source repository kind. Supported values: `git`,`file`,`oci`.

If left empty materia will guess based off the provided URL. Otherwise the specified `source.url` will be provided directly to the source provider.

#### *MATERIA_SOURCE__URL* / **source.url**

Source location of the `materia-repository(5)` in URL format. Will be provided directly to the source provider.

## See Also

`materia-source-git(5)`, `materia-source-file(5)`, `materia-source-oci(5)`
