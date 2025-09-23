---
title: MATERIA-CONFIG-GIT
section: 5
header: User Manual
footer: materia 0.1.0
date: June 2025
author: stryan
---

## Name
materia-config-git - Materia configuration for Git based source management

## Synopsis

**$MATERIA_GIT_<option-name>**

## Description

Settings for Git based source repositories.

## Options

#### **MATERIA_GIT_BRANCH**/ **git.branch**

Git branch to checkout.

#### **MATERIA_GIT_PRIVATE_KEY**/ **git.private_key**

Private key used for SSH-based git operations

#### **MATERIA_GIT_USERNAME**, **MATERIA_GIT_PASSWORD**/ **git.username/git.password**

Username and password used for HTTP-based git operations

#### **MATERIA_GIT_KNOWNHOSTS**/ **git.knownhosts**

`knownhosts` file used for SSH-based git operations. Useful if you're running materia in a container.

#### **MATERIA_GIT_INSECURE**/ **git.insecure**

Disable SSH knownhosts checking for git operations.
