# Example Materia Repository

This is an example Materia repository. It can be used as a reference while creating your own repository.

A Materia repository contains the components and secrets used by your hosts. For example, if you were to run `materia update` on host `ivy`, here's what would happen:

This assumes all default file paths are used.

1. Materia would clone this repository to `/var/lib/materia/source`, or run `git pull` if the repository is already cloned.
2. It would check the repository's `MANIFEST.toml` to see what components were assigned to `ivy`
3. It would see `ivy` has the `base` role assigned to it. The `base` role contains the `beszel-agent` component.
4. It would copy all files in `./components/beszel-agent/` to their destination places. In this case, it's just a single quadlet resource `beszel-agent.container.gomtpl`, which would be installed as `/etc/containers/systemd/beszel-agent/beszel-agent.container`.
5. It would start all services listed in `./components/beszel-agent/MANIFEST.toml`. In this case, it's just a single service `beszel-agent.service`.

## Repository manifest
The file `./MANIFEST.toml` is called the **repository manifest** or **Materia manifest**. It holds metadata about the repository; namely what **secrets engine** to use and what **components** are assigned to each host or **role**.

Every repository must have a manifest, even if it's empty.

## Components
The `./components/` directory contains the bread and butter of a Materia repository, the **components**. Components are collections of files (or **resources** ) that are installed and removed together. They also contain a `MANIFEST.toml` file, containing metadata like what services should be started and what default **secrets** the component contains.

Every component **must** have a manifest file, even if it's empty.

This repository contains the following components:

```
# beszel-agent and beszel-sever are two "real" components in that they represent what an actual component would like like.
./components/beszel-agent/
./components/beszel-server/
# hello is an example component that demonstrates as many resource types and other settings as possible for a component.
./components/hello/
```

## Attributes

**Attributes** are variables in a resource. Attributes are templated at run time and can be stored either as a `default` in the Component manifest or in the repository's **attibutes engine**. Defaults are stored unencrypted, while the attributes engines are usually encrypted.

For example, a common attribute is the `containerTag` attribute, representing what container tag the quadlet should use. It is often found in resources like so:

`Image=docker.io/henrygd/beszel:{{.containerTag}}`

Components usually define a default value like so:

`components/beszel-agent/MANIFEST.toml`:
```
[Defaults]
containerTag = "latest"

```

The `containerTag` variable can also be set in the attributes engine by creating a file `attributes/vault.toml` with the content:
```
[components.beszel-agent]
containerTag = "latest"
```

This will overwrite the `defaults` value, if one is set.

This repository contains example tooling in the `mise.toml.example` file for encrypting `attributes/*.toml` files as `attributes/*.age` files using `mise` tasks. You do not need to do this and can manage age files (or any other attributes engine) using whatever third party tooling you wish.
