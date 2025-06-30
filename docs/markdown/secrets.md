# Secrets

Materia uses secrets to handle configuration differences between hosts and environments. This is commonly used to control basic variables like "what container tag should be used for this host" and inject configuration values on a per-machine basis.

A **secrets engine** refers to how the secrets are either stored or made accessible to each host. Materia currently supports two secrets engines: **age** and **file**.

Secrets are usually stored in a **vault**; for file-based engines like **age** , this usually refers to the specific encrypted files.

Materia does not provide tools for managing secrets outside of what is needed at run time. For example, materia does not have a method of creating age-encrypted vaults, only reading them.

## Secrets Vault Types

Vaults come in three types: **global**, **host**, and **role**.

Global vaults contain secrets available to all hosts and components.

Host vaults contain secrets available to all components on a host.

Role vaults container secrets available to all hosts with the assigned role.

## Secrets Engines

### Age (recommended)

[Age](https://github.com/FiloSottile/age) is a modern public-key encryption system for files. It is the default encrypted secrets option.

### File

The file engine uses flat, unencrypted TOML files. It is suitable for usage if you don't need encryption or are just testing.


## Configuration locations

Secrets engines are unique in that they can be configured in three separate places: the repository MANIFEST, the materia config file, and through environmental variables. It is recommended you configure your engine through the manifest file so that it is the same for each host.

Secrets engine configuration value precedences follows the same general rule of "Least specific to Most specific": Manifest is overwritten by -> Config file is overwritten by -> Environmental Variable.

Each secrets engine has it's own configuration values that can be viewed on their individual reference pages.

