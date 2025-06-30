---
title: ATHANOR
section: 1
header: User Manual
footer: materia 0.1.0
date: June 2025
author: stryan
---

# NAME

athanor - a container backup solution for materia

# Synopsis
**athanor** [**--materia** *MATERIA_CONFIGFILE*] [**--athanor** *ATHANOR_CONFIGFILE*] backup

# Description
Athanor is a backup utility designed to backup quadlet volumes managed by Materia.

Athanor is pre-alpha quality software and should not be relied on for production usage.

# Global Flags
- `--materia config, -m`: Specify materia config file
- `--athanor config, -a`: Specify athanor config file

### Commands

#### `backup`
**Usage**: `athanor backup`
**Description**: Backup all materia-managed volumes

#### `version`
**Usage**: `athanor version`
**Description**: Display version information with git commit hash

