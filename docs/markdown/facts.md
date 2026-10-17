# Facts

Materia gathers some facts about the host as part of it's plan creation. These facts can be queried at runtime with the `materia facts` command, or used in a template with the `{{ m_facts "factname" }}` macro.

## Fact format

Facts are denoted as dotted paths, separated by namespace. For example, `materia.hostname` returns the hostname as `materia` knows it (i.e. including config overrides). Meanwhile, `host.hostname` returns the `host` namespace hostname fact (i.e. the results of `os.Hostname()`).

## Default facts:
The following facts are available:

### host
* `hostname`: the hostname for the current host, as provided by the OS

* `interface`: The ip addresses associated with the given interface. Used in the format `interface.INTERFACENAME.IPTYPE.ADDRESS_NUMBER`
    For example: `host.interface.tailscale0.ip4.0` would return the first IP address associated with the `tailscale0` interface.

### materia

* `hostname`: the hostname for the current host, as `materia` sees it (with config overrides).

* `roles`: what roles are assigned to the host

* `assigned_components`: what components are assigned to the host

* `installed_components`: what components are currently installed.

* `last_revision`: The last revision applied. Relies on `materia`'s output directory existing (by default `MATERIA_DIR/output`). This fact existing does not mean the revision was fully applied, just that it is the last attempted.

* `last_run`: The last run time. Relies on `materia`'s output directory existing (by default `MATERIA_DIR/output`)

* `last_run_succesful`: Whether the last run was successful e.g. all expected steps were completed
