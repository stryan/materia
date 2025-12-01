# Materia Repository

A Materia Repository is the source of truth for a deployment. It contains a Materia Manifest describing what components belong to which host, any remote Components needed, and other metadata for the deployment.

The repository is cloned from the configured [source](./reference/materia-source.5.md) as the first step of most Materia operations.

Repositories are usually stored in Git but can also be another local file location.

## Materia Manifest

Also known as a repository manifest, this `MANIFEST.toml` file describes component assignments and other metadata. All repositories **must** have a manifest file.

A simple manifest might look like this:

```toml
[hosts]
[hosts.vindicta]
components = ["freshrss"]
roles = ["base"]

[roles]
[roles.base]
components = ["podman_exporter"]


```


This defines two entities: a `host `and a `role`. Roles are collections of components and are assigned to hosts. Any host with a given role will be treated as if it has all the role's assigned components assigned to it.

In this case we have one host: `vindicta`. Vindicta has the "freshrss" component directly assigned to it. Since it has the "base" role it will also have all of those components assigned to it: in this case "podman_exporter".

Materia manifets can also include other metadata or orchestration configuration. For example, we can override a components defined services with the `Overrides` key:

```toml
[hosts.vindicta]
components = ["freshrss"]
[hosts.vindicta.overrides.freshrss.Services]
```

This override will replace the `Services` array defined in the `freshrss` component with the one in the manifest. In this case, we're simply replacing it with an empty array so no services are started.

For more details about the Materia manifest, see the [reference page](./reference/materia-manifest.5.md)
