# Facts

Materia gathers some facts about the host as part of it's plan creation. These facts can be queried at runtime with the `materia facts` command, or used in a template with the `{{ m_facts "factname" }}` macro.

The following facts are available:

* `hostname`: the hostname for the current host

* `interface`: The ip addresses associated with the given interface. Used in the format `interface.INTERFACENAME.IPTYPE.ADDRESS_NUMBER`
    For example: `interface.tailscale0.ip4.0` would return the first IP address associated with the `tailscale0` interface.
