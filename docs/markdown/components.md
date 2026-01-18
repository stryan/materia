# Components

Components are the basic building block of a Materia repository. They are analogous to Ansible roles, Puppet modules, or ArgoCD Applications.

Components consist of **Resources** and a **Component Manifest**, and are organized as a directory tree rooted in the `components/` folder of your repository.

The Component name is the name of the directory it's in.

For example purposes, the following file layout will be used:

```
components/
components/hello/
components/hello/MANIFEST.toml
components/hello/hello.container
components/hello/hello.env
components/hello/conf/config.toml
```

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

```toml
Defaults.containerTag = "latest"

[[Services]]
Service = "hello.service"
RestartedBy = "hello.container"
ReloadedBy = "conf/config.toml"
```

This manifest sets the default "containerTag" attribute to be "latest" and defines the "hello.service" service.

When the component is installed and when `materia update` is run, the "hello.service" systemd unit will be started or restarted.

When the "hello.container" resource is updated, the "hello.service" unit will be restarted.

When the "conf/config.toml" resource is updated, the "hello.service" unit will be reloaded. Note the resource is given by its relative path to the component root.

See the [Manifests reference](./reference/materia-manifest.5.md) for more details.
