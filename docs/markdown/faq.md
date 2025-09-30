# FAQ

## Why should I use this over Ansible/Puppet/Chef/some other configuration management?

Short answer: Because writing complicated YAML playbooks and Ruby modules is a pain and it's way easy to just drop a few unit files in a Git repo.

Longer answer: This is an opinionated tool with a tighter field of concern (applications/servers managed by Quadlets instead of full system state) it can afford to do much more without user intervention.
For example, when the services list for a component changes Materia will automatically stop the old services without the user having to specify.
It also supports tighter integrations with container native features like volume backups and (in the near future) recreating volumes and auto-migrating data.

## Why should I use this over Kubernetes?

Because you don't have a cluster and/or you probably don't need one.

Materia is designed for smaller-scale deployments. It is not intended to replace the full scale of Kubernetes or any similar software.

At some point I'd like to look into adding BlueChi integration so that components can have dependencies on other nodes, but that's far in the future.

## Can I manage my whole system with this?

Probably not. Materia is specifically designed to handle the application level; it does not and will not do stuff like my create or remove users, add firewall rules, etc. I highly encourage using *materia* along with something like Ansible or Terraform to handle the rest of the systems needs.

The rough intention is that this tool is deployed on top of an atomic distro like **OpenSUSE MicroOS** or **Fedora CoreOS**, where the majority of the system is controlled as a read-only image and the application servers (like app backends and `nginx`) are run as containers in the read/write section.
While traditional distros are a supported platform, most design work and testing is done assuming one of the above.

## Will more attributes backends be supported besides age and sops?

Yes. I hope to add other attributes backends like Vault soon; `age` and `sops` were just easy first targets and work well with the GitOps style.

On a related note, management tools like `ansible-vault` are not within scope of the project. It is expected to manage attributes entirely with a third party tool.

## Is this related to Final Fantasy?

No, it's named after the alchemical concept of the [prima materia](https://en.wikipedia.org/wiki/Prima_materia). All of the materia-related tools are named after alchemical terms.

Also the last good Final Fantasy game was FF3 and I will die on this hill.

## Is this related to \<insert any of the several companies named materia\>

No, but it's funny we all get out-SEO'd by a video game.
