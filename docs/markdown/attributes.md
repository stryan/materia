# Attributes

Materia uses **attributes** to handle configuration differences between hosts and environments. This is commonly used to control basic variables like "what container tag should be used for this host" and inject configuration values on a per-machine basis.

An **attributes engine** refers to how the attributes are either stored or made accessible to each host. Materia currently supports three attributes engines: **age**, [**sops**](https://github.com/getsops/sops), and **file**.

Attributes are stored in a **vault**; for file-based engines like **age** or **sops**, this usually refers to the specific encrypted files.

Materia does not provide tools for managing attributes outside of what is needed at run time. For example, materia does not have a method of creating age-encrypted vaults, only reading them.

## Attributes Vault Types

Vaults come in three types: **global**, **host**, and **role**.

Global vaults contain attributes available to all hosts and components.

Host vaults contain attributes available to all components on a host.

Role vaults container attributes available to all hosts with the assigned role.

## Attributes Engines

### Age (recommended)

[Age](https://github.com/FiloSottile/age) is a modern public-key encryption system for files. It is the current recommended encrypted secrets option because it is simple and easy to use.

Materia expects Age-encrypted files to be TOML files.

### SOPS (in-testing)

[SOPS](https://github.com/getsops/sops) is a editor and system for storing encrypted key value data. It also supports Age based encryption and encrypting only the values, which makes it easier to see what has changed.

Materia expects SOPS-encrypted files to be either YAML or INI files.

SOPs is still in testing but will most likely become the new recommended attributes engine.

### File

The file engine uses flat, unencrypted TOML files. It is suitable for usage if you don't need encryption or are just testing.


## Configuration locations

Attributes engine configuration value precedences follows the same general rule of "Least specific to Most specific": Config file is overwritten by -> Environmental Variable -> CLI flags.

Each attributes engine has it's own configuration values that can be viewed on their individual reference pages.

