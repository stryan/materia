
---
title: MATERIA-REPOSITORY
section: 5
header: User Manual
footer: materia 0.1.0
date: June 2025
author: stryan
---

# Name
*materia-repository*

# Synopsis

A directory containing containing components and manifests for materia to manage.

# Description

A directory containing containing components and manifests for materia to manage. Example file layout:

```
materia-repo/
materia-repo/components
materia-repo/components/hello
materia-repo/components/hello/hello.container.gotmpl
materia-repo/components/hello/MANIFEST.toml
materia-repo/secrets
materia-repo/secrets/vault.age
materia-repo/MANIFEST.toml
```

# Details

**component**

:  A collection of one or more resources, where at least one resource is a `MANIFEST.toml`. Components are assigned to hosts or roles in the repositories MANIFEST.toml.

   Example: `materia-repo/components/hello`

**resource**

:  A single file in a component. Resources are either static files or Golang templates and come in the following types:

   **file**: An arbitrary data file. Installed to `PREFIX/components/COMPONENT_NAME/RESOURCENAME`

   *service*: A systemd unit file. Installed to `PREFIX/components/COMPONENT_NAME/RESOURCENAME` and `MATERIA_SERVICEDIR`

   *container*: A `.container` file. Installed to `MATERIA_QUADLETDIR.`

   *volume*: A `.volume` file. Installed to `MATERIA_QUADLETDIR.`

   *pod*: A `.pod` file. Installed to `MATERIA_QUADLETDIR.`

   *kube*: A `.kube` file. Installed to `MATERIA_QUADLETDIR.`

   *manifest*: A `MANIFEST.TOML` file. Installed to `PREFIX/components/COMPONENT_NAME/MAINFEST.TOML`

   *volumefile*: A data file that should be installed in a Podman volume. Experimental, defined in the components `MANIFEST.toml`

   *script*: A script file, ending in `.sh`. Installed in `PREFIX/components/COMPONENT_NAME/RESOURCENAME `as well as `MATERIA_SCRIPTSDIR`

   *componentscript*: A special script file named either `setup.sh` or `cleanup.sh`. The former is run when the component is installed and the latter on removal.

   If a resource filename ends in `.gotmpl` it is treated as a Golang template.

   Example: `materia-repo/components/hello/hello.container.gotmpl`

**variable**

:  A Golang template variable. Usually defined as either a `secret` or in the `defaults` section of a component manifest

**secret**

:  A Golang template variable stored encrypted in the repository, like the default `age` encryption.

   Secrets are usually stored in a subdirectory `materia-repo/secrets`. There are three main types of secrets files:

   `secrets/vault.(toml|age)`: A general list of secrets available to all components, hosts, and roles.

   `secrets/<hostname>.(toml|age)`: Secrets available only to a specific host

   `secrets/<role>.(toml|age)`: Secrets available to a specific role

