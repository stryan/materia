---
title: MATERIA-CONFIG-FILE
section: 5
header: User Manual
footer: materia 0.4.4
date: December 2025
author: stryan
---

## Name
materia-config-file - Materia configuration for file based attribute management

## Synopsis

**$MATERIA_FILE__<option-name>**

## Description

Settings for file based attribute management.

If you don't need any settings (i.e. you're using the default vaults and base dir), you can enable the engine by setting `MATERIA_FILE=""` or adding an empty `[file]` table to your config.

Supports TOML files.

## Options

#### **MATERIA_FILE__BASE_DIR**/**file.base_dir**

Directory that contains attributes. Defaults to `secrets`.

#### **MATERIA_FILE__VAULTS**/**file.vaults**

Files that are general attributes vaults. Defaults to `vault.toml`.
