---
title: MATERIA-TEMPLATES
section: 5
header: User Manual
footer: materia 0.1.0
date: June 2025
author: stryan
---

## Name
materia-templates - Variables, functions, and snippets accessible within Materia template execution

## Description

Materia uses the standard Go templating engine for executing resources that end in the `.gotmpl` file type.

To access a secret within a templated resource, use the standard Go Template method of accessing variables: `{{ .containerTag }}` will resolve to the `containerTag` secret.

Besides normal secrets insertion, Materia also supports **macros** and **snippets**.

## Macros

Macros are privileged functions available to Materia while executing a resource. They are not user-modifiable.

Macros are accessed the same way as any other Go Template function: `{{ macro_name }}`.

### Macro List

#### **m_dataDir component_name**

Reference the components **data directory**. Often used for templating bind mounts

   Example: `Volume={{ m_dataDir "component" }}/config.yaml:/config.yaml` will be templated as `Volume=/var/lib/materia/components/component/config.yaml:/config.yaml`

#### **m_facts fact_name**

Lookup a fact about the host.

   Example: `PublishPort={{ m_facts "interface.tailscale0.ip4.0" }}:{{.port}}:{{.port}}` would template as `PublishPort=<tailscale interface IP address>:<port secret>:<port secret>`

#### **m_default secret value**

Return a secret's value or the provided value if the secret is not defined

#### **exists secret**

Returns true if the secret is defined, otherwise false

#### **snippet "snippet_name" "argument"**

Special macro, see the Snippets section below

#### **secretEnv "secret name" "TARGET (OPTIONAL)"**

Access a Materia secret that is specified as a podman secret in the component manifest. The "secret name" should be as specified in the `secrets = ["secret_name"]`. Optionally, provide the target as defined in the Podman manual

#### **secretMount "secret name" "ARGS (OPTIONAL)"**

Same as `secretEnv` but accesses the secret as a file mount. Optionally, provide additional arguments as defined in the Podman manual


#### **m_deps**

Legacy macro for generating default unit dependencies


## Snippets

Snippets are pre-made blocks of templated text that can be inserted with the `snippet` macro. Some come with materia, while others are defined in a component manifest or Repository manifest.

Snippets are not designed for highly-dynamic text, but can take one value as an argument.

Snippets are an experimental feature

##### Built-in Snippets

**autoUpdate <update_source>**

`Label=io.containers.autoupdate=<update_source>`



