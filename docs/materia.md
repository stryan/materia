## Materia

### Overview
Materia is a tool for managing quadlet files.


### Global Flags
- `--config, -c`: Specify TOML config file (env: `MATERIA_CONFIG`)
- `--nosync`: Disable syncing for commands that sync (env: `MATERIA_NOSYNC`)

### Commands

#### `config`
**Usage**: `materia config`
**Description**: Dump the active configuration to stdout

#### `facts`
**Usage**: `materia facts [flags]`
**Description**: Display host facts and role information
**Flags**:
- `--host`: Return only host facts (no assigned roles)
- `--fact, -f <name>`: Lookup a specific fact by name

#### `plan`
**Usage**: `materia plan [flags]`
**Description**: Generate and display an application deployment plan
**Flags**:
- `--quiet, -q`: Minimize output
- `--resource-only, -r`: Only install resources
**Output**: Saves plan to `plan.toml` as well as outputs to stdout

#### `update`
**Usage**: `materia update [flags]`
**Description**: Plan and execute a complete update operation
**Flags**:
- `--quiet, -q`: Minimize output
- `--resource-only, -r`: Only install resources. Skips any service related commands (besides daemon-reload).
**Output**: Saves executed plan to `lastrun.toml` as well as outputting to stdout

#### `remove`
**Usage**: `materia remove <component>`
**Description**: Remove a specific component
**Arguments**:
- `component`: Name of the component to remove

#### `validate`
**Usage**: `materia validate [flags]`
**Description**: Validate components/repositories for given hosts/roles
**Flags**:
- `--component, -c <name>`: Component to validate
- `--source, -s <path>`: Repository source directory
- `--roles, -r <roles>`: Roles for facts generation (can be repeated)
- `--hostname, -n <hostname>`: Hostname for facts generation
- `--verbose, -v`: Show extra detail

#### `doctor`
**Usage**: `materia doctor [flags]`
**Description**: Detect and optionally remove corrupted installed components
**Flags**:
- `--remove, -r`: Actually remove corrupted components (default is dry run)

#### `clean`
**Usage**: `materia clean`
**Description**: Remove all related file paths and cleanup

#### `version`
**Usage**: `materia version`
**Description**: Display version information with git commit hash
