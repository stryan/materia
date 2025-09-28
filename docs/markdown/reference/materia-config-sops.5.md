---
title: MATERIA-CONFIG-SOPS
section: 5
header: User Manual
footer: materia 0.3.0
date: September 2025
author: stryan
---

## Name
materia-config-sops - Materia configuration for SOPS based attribute management

## Synopsis

**$MATERIA_SOPS_<option-name>**

## Description

**EXPERIMENTAL**

Settings for SOPS based attribute management.

These are in addition to the normal SOPs configuration settings.

Supports YAML,JSON, and INI files.

## Options

#### **MATERIA_SOPS_BASE_DIR**/**sops.base_dir**

Directory that contains attributes

#### **MATERIA_SOPS_VAULTS**/**sops.vaults**

Files that are general attributes vaults. Defaults to "vault.yml" and "attributes.yml".

#### **MATERIA_SOPS_SUFFIX**/**sops.suffix**

Suffix that denotes an encrypted file and comes before the base file type. Use this if your base directory includes both encrypted and un-encrypted files.

Example: `sops.suffix = "enc"` will cause materia to look for files like `vault.enc.yml` instead of `vault.yml`.
