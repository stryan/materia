# Components

Components are the basic building block of a Materia repository. They are analogous to Ansible roles, Puppet modules, or ArgoCD Applications.

Components consist of **Resources** and a **Component Manifest**, and are organized as a directory tree rooted in the `components/` folder of your repository.

The Component name is the name of the directory it's in.

For example purposes, the following file layout will be used:

~~~default
components/
components/hello/
components/hello/MANIFEST.toml
components/hello/hello.container
components/hello/hello.env
components/hello/conf/config.toml
~~~

This represents a component named "hello".

## Component Resources

These are the files that are installed as part of the component and are split between Quadlet and Data files.

Quadlet files are installed to a subdirectory in the Quadlet directory on the host. Data files are kept in the Materia data directory for the component.

For the example component, these directories would be `/etc/containers/systemd/hello` and `/var/lib/materia/components/hello`, respectively.

For data resources, the file layout of the component will be copied to the installation directory i.e. `components/hello/conf/config.toml` would be installed to `/var/lib/materia/components/hello/conf/config.toml`.

See the main [Resources](./resources.md) page for more details.

## Component Manifest

The Component Manifest contains metadata for the component. This includes default Attribute values,Podman Secrets, and what Services to start for the component.

All components **MUST** have a Manifest file to be considered a valid component. An empty Manifest file is a valid manifest.

Component Manifests can not be templated outside of the `Scripts` section.

For the example component, the Manifest file might look like this:

~~~toml
Defaults.containerTag = "latest"

[[Services]]
Service = "hello.service"
RestartedBy = [ "hello.container" ]
ReloadedBy = [ "conf/config.toml" ]
~~~

This manifest sets the default "containerTag" attribute to be "latest" and defines the "hello.service" service.

When the component is installed and when `materia update` is run, the "hello.service" systemd unit will be started or restarted.

When the "hello.container" resource is updated, the "hello.service" unit will be restarted.

When the "conf/config.toml" resource is updated, the "hello.service" unit will be reloaded. Note the resource is given by its relative path to the component root.

See the [Manifests reference](./reference/materia-manifest.5.md) for more details.

## Delegated Components

(EXPERIMENTAL)

A root materia installs every Quadlet in the system directory, so every container it manages is rootful. A component that sets `User` runs rootless instead, as that account:

~~~toml
[Settings]
User = "hello"
~~~

Two materia instances share the work:

| | Root materia | The account's materia |
|---|---|---|
| Syncs the repository, picks the host's components | yes | no |
| Applies overrides and extensions, decrypts attributes | yes | no |
| Renders templates | no | yes |
| Installs Quadlets, units, data files, Podman secrets | no | yes |
| Starts, stops and restarts services | no | yes |
| Runs `SetupScript`, `CleanupScript`, `PreScript`, `PostScript` | yes, as root | no |

Templates are rendered by the account's materia, so macros such as `m_dataDir` and `isRoot` give that account's values.

A component without `User` is installed exactly as before.

### What root materia does

Root materia keeps a record of the component in its data directory: the Manifest and the lifecycle scripts. It writes everything else to a staging directory, `<delegate_dir>/accounts/<user>/`, which is a materia repository:

~~~default
/var/lib/materia/delegate/accounts/hello/
    MANIFEST.toml                  generated: assigns every staged component
    components/hello/              the component, templates not rendered
    attributes/hello.toml          the attributes resolved for the component
~~~

Then it calls `Update` on the varlink socket of the account's materia server, `/run/user/<uid>/materia/materia.sock`.

The account owns that socket, so root materia does not trust what answers there:

- It sends nothing unless the peer runs as the account.
- It reads one reply of at most 64 KiB: a step count, or an error.
- It removes characters that do not print from the error text, and keeps at most 512 characters.
- It gives up after `executor.delegate_timeout` seconds.

The reply decides only whether root materia records the delivery. An account that reports success without applying anything is not detected.

Order for a new or changed component:

1. `PreScript` (changed components only)
2. Install or update the record
3. Stage the component
4. `PostScript` (changed components only)
5. `SetupScript` (new components only)
6. After every component in the plan: one update per account

Order for a removed component:

1. Remove the staged copy
2. Update the account, which stops and removes the component there
3. `CleanupScript`
4. Remove the record

Root materia plans a delegated component again until the account's materia has applied it. So a failed update is retried on the next run, including the scripts.

### Setting up the account

Root materia does not create the account and does not configure its materia. Do this once, for example in the component's `SetupScript`, which runs after the component is staged and before the first update:

1. Create the account, with subordinate ID ranges and lingering enabled.
2. Give the account read access to its staging directory. Materia creates it with mode `0750`, owned by root, and never changes it afterwards. For example: `chgrp hello /var/lib/materia/delegate/accounts/hello`. The mode of this directory is the only thing that keeps other users out: the files below it, including the attributes, are readable by everyone who can enter it.
3. Run `materia server` as the account, in its user manager, with a configuration like this:

~~~toml
attributes = "file"

[source]
kind = "local"
url = "file:///var/lib/materia/delegate/accounts/hello"

[file]
base_dir = "attributes"
load_all_vaults = true
~~~

Keep the configuration outside the account's home directory and pass it with `MATERIA_CONFIG`. Do not set `server.update_interval`: root materia decides when the account updates.

### Limits

- `materia plan` as root shows one `Delegate Account` step per account, not the account's own steps. To see those, run `materia agent plan` as the account.
- The staging directory holds the component's attributes in plain text. Only root and the account can read it.
- Root materia's rollback does not cover the account's services.
- The account owns its installed Quadlets and units. Root materia does not check them, and asks the account to update only when a component changed.
- Changing or removing `User` on an installed component is an error. Remove the component from the host first, then assign it again.
- Root materia finds the account in `/etc/passwd`.

## Component Instances

Component assignments can be instanced in the same way as systemd templates: `component@instance`. This allows Materia to install multile instances of the same source component.

This is useful for components that use systemd templating to provide multiple installations e.g. video game servers. Materia will perform the following resource path substitutions:

Example component `server@site1`
~~~default
     server/ -> server@site1/
     server@.container -> server@site1.container
~~~

Similarly, Materia will substitue `@` signs in `[[Services]]` definitions as well with the instance name:
~~~toml
[[Services]]
Service = "server@.container"
~~~

becomes

~~~toml
[[Services]]
Service = "server@site1.container"
~~~

Attribute templating for this component is likewise extended: attributes assigned to the `server` component are available to both `server@site1` and `server@site2`, but attributes assigned to `server@site1` are only available to `server@site1` and will override attributes assigned to `server`.
