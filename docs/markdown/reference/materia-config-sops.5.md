---
title: MATERIA-CONFIG-SOPS
section: 5
header: User Manual
footer: materia 0.5.0
date: January 2026
author: stryan
---

## Name
materia-config-sops - Materia configuration for SOPS based attribute management

## Synopsis

*$MATERIA_SOPS__<option-name>*

## Description

Settings for SOPS based attribute management.

These are in addition to the normal SOPs configuration settings.

If you don't need any settings (i.e. you're using the default vaults and base dir), you can enable the engine by setting `MATERIA_SOPS=""` or adding an empty `[sops]` table to your config.

Supports YAML,JSON, and INI files.

## Options

#### **MATERIA_SOPS__BASE_DIR**/**sops.base_dir**

Directory that contains attributes. Defaults to `secrets`.

#### **MATERIA_SOPS__VAULTS**/**sops.vaults**

Files that are general attributes vaults. Defaults to "vault.yml" and "attributes.yml".

#### **MATERIA_SOPS__LOAD_ALL_VAULTS**/**sops.load_all_vaults**

Whether to load all vault files that exist without filtering by role, or filename above. Defaults to `false`.

#### **MATERIA_SOPS__SUFFIX**/**sops.suffix**

Suffix that denotes an encrypted file and comes before the base file type. Use this if your base directory includes both encrypted and un-encrypted files.

Example: `sops.suffix = "enc"` will cause materia to look for files like `vault.enc.yml` instead of `vault.yml`.

## File Format

A file vault is a YAML or INI file with one or more of the following maps:

`globals`: Global attributes
`hosts`: Attributes scoped to a host
`components`: Attributes scoped to a component
`roles`: Attributes scoped to a role


An example file would look like this:

```yaml
globals:
    localDNS: 192.168.10.10
    localDomain: saintnet.lan
    tailscaleDomain: tail36717.ts.net
components:
    caddy:
        caddyImage: git.saintnet.tech/stryan/saintnet_caddy
```
