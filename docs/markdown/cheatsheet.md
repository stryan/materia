# Cheat Sheet

## Terminology

target: The node materia is running on

repository: A local directory or Git repository containing Materia components,manifests, and resources.

manifest: A TOML file containing metadata. Found at the repository level and the component level.

component: A collection of resources. Similar to an ansible role and is the basic building block of a repository.

resource: An individual file that is installed as part of a component and removed when the component is removed. Includes quadlets, data files, non-generated systemd units, scripts, and more.

execution: The act of Materia actually installing a resource to a target. For non-templated resources this is a straight file copy. For templated resources, this involves treating it as a Go template.

## Default locations

Base prefix for materia data: `/var/lib/materia`

Target's local copy of source repository: `/var/lib/materia/source`

Non quadlet files for components: `/var/lib/materia/components/COMPONENT_NAME`

Script resources: `/usr/local/bin/` and `/var/lib/components/COMPONENT_NAME`

Systemd unit files: `/etc/systemd/system/` and `/var/lib/components/COMPONENT_NAME`

Quadlet files (.container,.volume,etc): `/etc/containers/systemd/COMPONENT_NAME/`

## Materia high-level overview

1. Sync copy of source repo on target host (e.g. `git pull` in `/var/lib/materia/source` )
2. Determine what components are assigned to the target
3. Mark all installed components that are no longer assigned to the target for removal
4. For all components that are assigned to the target and already installed, check for any added or removed resources
5. For any resource that exists in both the source cache and the target host, check for any differences, templating if necessary.
6. Generate plan of installing/removing/updating resources and components.
7. If any components are changed, schedule a `systemctl daemon-reload`
8. Determine what services need to be stoped/started/restarted.
9. Start execution.
10. In alphabetical order, add/remove/update components.
11. If any components with setup/cleanup scripts are installed/removed, run those scripts.
12. Modify services as calculated in step 8.
13. Wait for services to start/stop
