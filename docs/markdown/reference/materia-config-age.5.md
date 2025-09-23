---
title: MATERIA-CONFIG-AGE
section: 5
header: User Manual
footer: materia 0.1.0
date: August 2025
author: stryan
---

## Name
materia-config-age - Materia configuration for Age based secrets management

## Synopsis

**$MATERIA_AGE_<option-name>**

## Description

Settings for Age based secret management.

## Options

#### **MATERIA_AGE_KEYFILE**/**age.keyfile**

File that contains the Age private key to use

#### **MATERIA_AGE_BASEDIR**/**age.base_dir**

Directory that contains secrets

#### **MATERIA_AGE_VAULTS**/**age.vaults**

Files that are general secrets vaults. Defaults to "vault.age" and "secrets.age".
