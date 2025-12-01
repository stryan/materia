---
title: MATERIA
section: 1
header: User Manual
footer: materia 0.3.0
date: September 2025
author: stryan
---

## NAME
materia - a tool for deploying Quadlets and associated files

## Synopsis

**materia** [**-h**] [**--config** *CONFIGFILE* ] [**--nosync**] *command*

## Description

**materia** is a tool designed for GitOps style management of services deployed via Podman Quadlets. It takes a URL pointing to a *repository* containing *manifests* and *components*, determines which components are assigned to the current host, and installs them while removing components that are no longer needed.

## Configuration

Materia can be configured via either environmental variables or a TOML config file. The most common environmental variables are documented below:

- `MATERIA_SOURCE__URL`: URL with location of the materia-repository. Example: `export MATERIA_SOURCE__URL="git://github.com/materia/materia_repo`

See materia-config(5) and materia-repository(5) for more details.

## Global Flags
- `--config, -c`: Specify TOML config file (env: `MATERIA_CONFIG`)
- `--nosync`: Disable syncing for commands that sync (env: `MATERIA_NOSYNC`)
- `--help, -h`: Show usage information

## Commands


#### *config*

Dump the active configuration to stdout in a human readable format

#### facts [flags]
Display host facts and role information

##### **Flags**

**--host** : return only *host* facts, instead of requiring a repository

**--fact, -f [factname]** :  Lookup a specific fact by name

#### plan [flags]
   Generate and display an deployment plan.

   Saves plan to `MATERIA_OUTPUT`/`plan.toml` as well as outputs to stdout (if quiet is not set).

##### **Flags**

**--quiet, -q**: Minimize output. Useful for validation that a plan can be generated.

**--resource-only, -r**: Only install resources instead of also starting/stopping services

**--format, -f**: Control output format. Supports json,text. Defaults text.


#### update [flags]
   Plan and execute a complete update operation.

   Saves executed plan to `MATERIA_OUTPUT`/`lastrun.toml` as well as outputting to stdout (if quiet is not set)

##### **Flags**

**--quiet, -q**: Minimize output

**--resource-only, -r**: Only install resources. Skips any service related commands (besides daemon-reload).

####  remove [component]
Remove a specific component. Note this does not remove it from the repository manifest.

##### **Arguments**:

**component**: Name of the component to remove

#### validate [flags]
   Validates a plan can be generated for given hosts/roles.

##### **Flags**

**--component, -c <name>**: Component to validate. If not specified, use what is definined in the repositories `MANIFEST.toml`

**--source, -s <path>**: Repository source directory, overrides `MATERIA_SOURCE__URL`

**--roles, -r <roles>**: Roles for facts generation (can be specified multiple time for multiple roles)

**--hostname, -n <hostname>**: Manually assigned hostname

**--verbose, -v**: Show extra detail

#### server
Run materia in the foreground as a service process.

See the `server` section in materia-config(5) for configuration options


#### agent
Run commands against materia server over a unix socket

##### Arguments

**--socket, -s <path>**: Manually specify a socket path. If not specified, defaults to `/run/materia/materia.sock` for root or `/run/uid/materia/materia.socket` for user.

##### Subcommands

**facts**: Return list of host facts

**sync:** Perform a repository sync

**plan:** Generate a plan

**update:** Run an update

#### doctor [flags]
Detect and optionally remove corrupted installed components.

##### **Flags**

**--remove, -r**: Actually remove corrupted components (default is dry run)

#### clean
Remove all related file paths and cleanup

#### version
Display version information
