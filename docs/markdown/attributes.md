# Attributes

Materia uses **attributes** to handle configuration differences between hosts and environments. This is commonly used to control basic variables like "what container tag should be used for this host" and inject configuration values on a per-machine basis.

An **attributes engine** refers to how the attributes are either stored or made accessible to each host. Materia currently supports three attributes engines: **age**, [**sops**](https://github.com/getsops/sops), and **file**.

Attributes are stored in a **vault**; for file-based engines like **age** or **sops**, this usually refers to one or more encrypted files.

Materia does not provide tools for managing attributes outside of what is needed at run time. For example, materia does not have a method of creating age-encrypted vaults, only reading them.

## Attributes Vaults

### Attribute Scoping

Attributes can be scoped to three different levels: **global**, **host**, **role**, and **component**.

**Global** attributes are available when templating any component on any host.

**Host** attributes are available when templating any component on the specified host.

**Role** attributes are available when templating any component on a host that belongs to the specified role.

**Component** attributes are available when templating the specified component.

### Vault Scoping

Similarly, vaults are scoped in three ways: *global*, *host*, and *role*.

This is determined by the *filenames* of the vault files:

*Global* vaults are named either `vault.(toml|yml)` or `attributes.(toml|yml)`, with the filetype depending on the engine used. Attributes here can be used on any host or with any component as configured by their **attribute scope**.

*Host* vaults are named after the hostname they are scoped to i.e. `localhost.toml`. Attributes here can be used on the host specified with any component as configured by their **attribute scope**

*Role* vaults are named after the role they are scoped to i.e. `base.toml`. Attributes here can be used on any host with the role specified, with any component as configured by their **attribute scope**

### Scoping Example:

The following example assumes you're using the [sops](#sops-recommended) engine.

The recommended file layout for organizing attributes is this:

```
attributes/
attributes/vault.yml
attributes/localhost.yml
attributes/base.yml
```

The `vault.yml` file includes attributes that are either referenced globally or are scoped to components but don't depend on the target host.

```yaml
globals:
    lanDnsServer: 192.168.1.10
    lanName: example.lan
components:
    caddy:
        caddyImage: docker.io/user/special_caddy_version
```

The `localhost.yml` file includes attributes that are specific to the host.

```yaml
globals:
    hostIP: 192.168.1.22
components:
    caddy:
        containerTag: stable
        caddyConfig: |
            {}
```


The `base.yml` file includes attributes for any host with the `base` role assigned to it.

```yaml
components:
    beszel-agent:
        beszelKey: ssh-blahimakey
```


## Attributes Engines

### SOPS (recommended)

[SOPS](https://github.com/getsops/sops) is a editor and system for storing encrypted key value data. It also supports Age based encryption and encrypting only the values, which makes it easier to see what has changed.

Due to its flexibility and existing tools, SOPS is the current recommended attributes engine.

Note: since SOPS is configured externally, you may not need to supply any custom configuration to Materia. To make sure Materia attempts to use SOPS vaults you can force usage with the `MATERIA_ATTRIBUTES` setting, or by providing a blank configuration with `[sops]` in the config file or with `export MATERIA_SOPS=""`

Materia expects SOPS-encrypted files to be either YAML or INI files.

An example SOPS vault with all four levels of **attribute scoping** looks like this:

```yaml
globals:
    localIP: 192.168.1.67
components:
    freshrss:
        domain: example.com
roles:
    base:
        bsezelKey: ssh-blahimadifferentkey
hosts:
    localhost:
        keyForLocalhost: value
```


### Age (recommended)

[Age](https://github.com/FiloSottile/age) is a modern public-key encryption system for files. It is a recommended encrypted secrets option because it is simple and easy to use.

Materia expects Age-encrypted files to be TOML files.

An example Age vault with all four levels of **attribute scoping** looks like this:

```toml
[globals]
    localIP = "192.168.1.67"
[components.freshrss]
    domain = "example.com"
[roles.base]
    bsezelKey = "ssh-blahimadifferentkey"
[hosts.localhost]
    keyForLocalhost = "value"
```


### File

The file engine uses flat, unencrypted TOML files. It is suitable for usage if you don't need encryption or are just testing.

The file engine uses the same TOML format as the Age engine.

## Configuration locations

Attributes engine configuration value precedences follows the same general rule of "Least specific to Most specific": Config file is overwritten by -> Environmental Variable which are overwritten by -> CLI flags.

Each attributes engine has it's own configuration settings that can be viewed on their individual reference pages.
