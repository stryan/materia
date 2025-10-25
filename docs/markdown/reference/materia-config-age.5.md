---
title: MATERIA-CONFIG-AGE
section: 5
header: User Manual
footer: materia 0.3.0
date: October 2025
author: stryan
---

## Name
materia-config-age - Materia configuration for Age based attribute management

## Synopsis

**$MATERIA_AGE__<option-name>**

## Description

Settings for Age based secret management.

## Options

#### **MATERIA_AGE__KEYFILE**/**age.keyfile**

File that contains the Age private key to use

#### **MATERIA_AGE__BASEDIR**/**age.base_dir**

Directory that contains attributes. Defaults to `secrets`.

#### **MATERIA_AGE__VAULTS**/**age.vaults**

Files that are general attribute vaults. Defaults to "vault.age" and "attributes.age".
