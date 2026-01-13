---
title: MATERIA-CONFIG-AGE
section: 5
header: User Manual
footer: materia 0.5.0
date: January 2026
author: stryan
---

## Name
materia-config-age - Materia configuration for Age based attribute management

## Synopsis

**$MATERIA_AGE__<option-name>**

## Description

Settings for Age based secret management.

If you don't need any settings, you can enable the engine by setting `MATERIA_AGE=""` or adding an empty `[age]` table to your config.

## Options

#### **MATERIA_AGE__KEYFILE**/**age.keyfile**

File that contains the Age private key to use. Defaults to `/etc/materia/key.txt`.

#### **MATERIA_AGE__BASE_DIR**/**age.base_dir**

Directory that contains attributes. Defaults to `secrets`.

#### **MATERIA_AGE__VAULTS**/**age.vaults**

Files that are general attribute vaults. Defaults to "vault.age" and "attributes.age".
