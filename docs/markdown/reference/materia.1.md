---
title: MATERIA
section: 1
header: User Manual
footer: materia 0.7.1
date: August 2026
author: stryan
---

## NAME
materia - a tool for deploying Quadlets and associated files

## Synopsis

**materia** [**-h**] [**--config** *CONFIGFILE* ] [**--nosync**] *command*

## Description

**materia** is a tool designed for GitOps style management of services deployed via Podman Quadlets. It takes a URL pointing to a *repository* containing *manifests* and *components*, determines which components are assigned to the current host, and installs them while removing components that are no longer needed.

**materia** works using a plan-execute pattern. It will generate a plan that includes all resources that need to be installed/removed/updated, all services that need to be started/stopped/restarted/enabled/disabled, and any other tasks required such as ensuring all needed Podman volumes are created. Executing the plan is broken up into roughly two stages:
   1. (Resource phase) Install/Update/Remove resources
   2. (Services phase) Start/Stop/Restart/Reload/Enable/Disable services

These two stages are usually separated by a "host reload" where **materia** executes a `systemctl daemon-reload`

Note that some service actions can take place during the Resource phase if required. For example, migrating data between volume resources on a volume quadlet update requires several systemd services to be started and stopped during the resources phase.

## Configuration

Materia can be configured via either environmental variables or a TOML config file. The most common environmental variables are documented below:

- `MATERIA_SOURCE__URL`: URL with location of the materia-repository. Example: `export MATERIA_SOURCE__URL="https://github.com/materia/materia_repo`

See materia-config(5) and materia-repository(5) for more details.

## Global Flags
- `--config, -c`: Specify TOML config file (env: `MATERIA_CONFIG`)
- `--nosync`: Disable syncing for commands that sync (env: `MATERIA_NOSYNC`)
- `--help, -h`: Show usage information

## Commands


#### *config*

Dump the active configuration to stdout in a human readable format

#### facts [flags]
Display facts known to materia, such as hostname, network interfaces, component status, etc.

##### **Flags**

**--fact, -f [factname]** :  Lookup a specific fact by name

#### plan [flags]
   Generate and display an deployment plan.

   Saves plan to `MATERIA_OUTPUT`/`plan.toml` as well as outputs to stdout (if quiet is not set).

##### **Flags**

**--quiet, -q**: Minimize output. Useful for validation that a plan can be generated.

**--resource-only, -r**: Only install resources. Skips any service related commands (besides daemon-reload).

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
   (EXPERIMENTAL)

   Validates a plan can be generated for a hostname. You can specify a specific component or role(s) to test, otherwise defaults to full repo.

   Caveat: Since this may be run on a host with different services then the live host, service information is unreliable.

   Note: Outside of the overrides specified below, this command will use whatever is current configured for materia in your current environment.
   Make sure you can access whatever attributes vaults you need locally.


##### **Flags**

**--hostname, -n <hostname>**: Required. Hostname to use

**--component, -c <name>**: Component to validate. If not specified, use what is defined in the repositories `MANIFEST.toml` for the given host/roles combination

**--source, -s <path>**: Repository source directory, overrides `MATERIA_SOURCE__URL`

**--roles, -r <roles>**: Roles for facts generation (can be specified multiple time for multiple roles).

**--verbose, -v**: Show extra detail

#### server
Run materia in the foreground as a service process.

See the `server` section in materia-config(5) for configuration options


#### agent
Run commands against materia server over a unix socket

##### Arguments

**--socket, -s <path>**: Manually specify a socket path. If not specified, defaults to `/run/materia/materia.sock` for root or `/run/uid/materia/materia.sock` for user.

##### Subcommands

**facts**: Return list of host facts, accepts same arguments as `facts` command.

**sync:** Perform a repository sync

**plan:** Generate a plan

**update:** Run an update

#### doctor [flags]
Detect and optionally remove corrupted installed components.

##### **Flags**

**--remove, -r**: Actually remove corrupted components (default is dry run)

#### clean [flags]
Remove all related file paths and cleanup. Attempts to plan and execute removals for all installed components first.

##### Flags

**--force, -f**: Do not attempt to gracefully remove components before cleaning up filepaths.

#### version
Display version information
