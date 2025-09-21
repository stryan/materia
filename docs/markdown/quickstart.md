# Quickstart

## Introduction

In this quickstart we will install the [caddy](https://caddyserver.com/) service on a host named `ivy`.

For [secrets](secrets.md) management we will use the default [age](https://github.com/FiloSottile/age) based encrypted vaults. If you do not wish to setup age encryption, you can skip the steps marked `OPTIONAL-AGE` and use ` secrets = "file"` in your `MANIFEST.toml`.

## Installation

### Building and installing from source (OPTIONAL)

(OPTIONAL - skip this section if you're using a container or pre-made release)

Clone the [repository](https://github.com/stryan/materia) and run `mise build` to generate binaries. If you do not wish to install [mise](https://mise.jdx.dev/) you may build it with the normal go tooling: `go build -ldflags="-w -s" -o bin/materia ./cmd/materia/`.

Assuming you use `mise` to build, this will have generated binaries for `amd64` and `arm64` in the format `bin/materia-<architecture>`.

Copy the binary to the destination host: `scp bin/materia-amd64 root@ivy:/usr/local/bin/materia`

Verify it works by running `materia version`.

## Building your repository

Like any GitOps tool, Materia needs a source repository that describes the desired state for your nodes. For the sake of this quickstart we're going to assume you already know how to create and push to a Git repository.

### Create the base materia repo

Create a git repository on your workstation named whatever you like; we're going to assume it's called `materia-repo`.

Inside the git repo, create the base directories:

`mkdir components secrets`

Finally, create a [manifest](reference/materia-manifest.5.md) file:

`touch MANIFEST.toml`

This manifest file will describe what components go on what hosts. We'll get back to it shortly.

### Create the caddy component

**Components** are what materia uses to refer to a service and its associated resources (config files,data files, etc). They are the basic building blocks of a Materia repository and are similar to an ansible role.

For this example we will create a component for the `caddy` service.

Components are organized by directories under the `components/` directory. Create a directory for `caddy`:

```
mkdir components/caddy
```

### Create the resources for Caddy

Caddy requires the following resources:

- a Container Quadlet resource to run the caddy container
- Two Volume Quadlet resources for caddy's data and config volumes
- a File resource for caddy's config file
- a Manifest file that describes the component's metadata. All components have a Manifest file, even if it's empty.

The next few steps are done in `components/caddy`:

`cd components/caddy`

#### Create the Caddy container Quadlet resource

Create a file `caddy.container.gotmpl` for the Caddy container Quadlet: `touch caddy.container.gotmpl`. We want to *template* secrets into the file later, so make sure the file ends with `.gotmpl` to designate it as a templated resource. Materia uses the standard [Go Templating engine](https://pkg.go.dev/text/template).

Using your editor of choice, insert the following lines into the file we just created:

```
[Unit]
Description=Caddy reverse proxy

[Container]
Image=docker.io/caddy:{{ .containerTag }}
ContainerName=caddy
AddCapability=NET_ADMIN
Volume=caddy-data.volume:/data
Volume=caddy-config.volume:/config
Volume={{ m_dataDir "caddy" }}/conf:/etc/caddy:Z

{{ snippet "autoUpdate" "registry" }}

# local web data
{{- if ( exists "localWeb" )}}
Volume={{ .localWeb }}:/srv/www:Z
{{- end }}
PublishPort=443:443


[Service]
ExecReload=podman exec caddy caddy reload --config /etc/caddy/Caddyfile

[Install]
# Start by default on boot
WantedBy=multi-user.target default.target
```

The text contained in brackets are Go Templating variables, e.g. `Image=docker.io/caddy:{{ .containerTag }}` would become `Image=docker.io/caddy:latest` if `containerTag` is set to `latest`.

Materia also includes some ease of use functions like `exists` and `m_dataDir` that are referred to as [macros](./reference/materia-templates.5.md).

Materia also includes pre-made [snippets](./reference/materia-templates.5.md) of text such as `"autoUpdate"`.

#### Create the data and config Volume resources

Create two files, `caddy-config.volume` and `caddy-data.volume`. Since these will not be templated files, they do not need to end in `.gotmpl`.

`touch caddy-config.volume caddy-data.volume`.


Both files should have the same content:
```
[Volume]
```

#### Create the config File resource

Now to create a `Caddyfile` for caddy's config. We are going to create this in an subdirectory that will be bind-mounted into the container; this is to make it easier for Caddy to see when the Caddyfile changes on `systemctl reload` and to show off how subdirectories will be translated as-is from the repository to the target host.

First create the subdirectory:

`mkdir conf`

Then create the Caddyfile template:

`touch conf/Caddyfile.gotmpl`

Insert the following line into your freshly created Caddyfile:

```
{{ .caddyfile }}
```

In the real world most config files are simple and can be templated more directly i.e. `ConfigOption = {{ .configValue }}` but since Caddy's is more complicated and this is a quickstart, we're going to store the entire configuration as a secret.

#### Create the Manifest resource

The last thing we need is a manifest file for the component. Create the file in the root of the component (i.e. `materia-repo/components/caddy/`)

`touch MANIFEST.toml`

Add the following content:

```
[Defaults]
containerTag = "latest"

[[services]]
Service = "caddy.service"
```

The `[defaults]` section is a TOML table describing default secret values. In this case, the `containerTag` secret is set to `latest` by default.

The `[[services]]` section is a TOML array describing what services the component cares about. They can be either a part of the component or installed seperately. In this case, the only service that is defined is the `caddy.service`. This means that when the component is installed materia will start the `caddy.service` unit, when the component is removed it will make sure the service is stopped,when the component has updated files it will restart the service, and if the service is detected as not-running when materia runs it will attempt to start it again.

For more details, such as how to set services to only restart when certain files are updated, see the [component section of the manifest reference](reference/materia-manifest.5.md).


### Create repository secrets

We've created the `caddy` component, but some of the resources in it use **secrets**. It's time to setup our secrets engine and store some secrets.

By default, the `age` secrets engine will look for `*.age` files named either `vault.age`, `secrets.age`, or the hostname of the node e.g. `ivy.age`.

We will be setting up two age-based **vaults** in our repository: one that applies to all hosts (`secrets/vault.age`) and one that only applies to `ivy` (`secrets/ivy.age`). For the age secrets engine, a vault is just an encrypted TOML file.

If you do not have the `age` cli tool installed, follow the instructions [here](https://github.com/FiloSottile/age/blob/main/README.md) to install it. While materia has age decryption built in it does not manage secrets vaults on its own and expects you to handle it.

#### Create the general vault

First we create the vault used by all hosts. Create a file named `secrets/vault.toml`:

`touch secrets/vault.toml`

Put the following content in it:

```
[global]
containerTag = "stable"

[components.caddy]
containerTag = "latest"
```

Secrets in the `[global]` section will be available to all hosts and all components.

Secrets in a `[components.componentname]` section will only be available to the component `componentname`.

With this file, all components with the `containerTag` secret will have the value `stable`, except for the `caddy` component which will have `latest`.

#### Create the host vault

Next, create a vault that will only apply to components installed on `ivy`:

`touch secrets/ivy.toml`

Insert the following content in it:

```
[components.caddy]
localWeb = "/srv/www"
```

This means the `localWeb` secret on `ivy` *and only ivy* will be set to "/srv/www".


#### Setup age based encryption (OPTIONAL-AGE)

1. Create an age key: `age-keygen -o key.txt`.
2. Extract the public key from the generated private key: `age-keygen -y key.txt`. Store this somewhere convenient.
3. Install the key on `ivy`; for this guide we'll put it in `/etc/materia/key.txt`. **DO NOT STORE THIS IN YOUR MATERIA REPOSITORY**
4. Encrypt the files to extracted public key: `cd secrets && for file in $(find  -name '*.toml'); do age -r <INSERT PUBLIC KEY HERE> -o $(basename $file .toml).age $file; done`
5. Once the files are successfully encrypted, remove the no longer needed raw TOML files: `rm secrets/*.toml`.


### Create the repository manifest

We have a component and we have secrets to use with the component, now to put it all together in the repository manifest.

Open the `materia-repo/MANIFEST.toml` file and add the following content:

```
secrets = "age"

[Age]
Keyfile = "/etc/materia/key.txt"
Basedir = "secrets"

[Hosts.ivy]
components = ["caddy"]
```

First we set what secrets engine we're using with `secrets = "age"`.

Then, we configure the `age secrets engine` by telling it what file on the host contains the age key (`identity`) and where in the repository to find secrets (`secrets`). These settings can also be configured at runtime or on the target node.

Finally, we define the `ivy` host (`[hosts.ivy]`) and assign the `caddy` component to it: `components = ["caddy"]`.


### Final results

The final repository directory should like this:

```
materia-repo/
materia-repo/components/
materia-repo/components/caddy
materia-repo/components/caddy/conf
materia-repo/components/caddy/conf/Caddyfile.gotmpl
materia-repo/components/caddy/caddy.container.gotmpl
materia-repo/components/caddy/caddy-config.volume
materia-repo/components/caddy/caddy-data.volume
materia-repo/secrets/
materia-repo/secrets/ivy.age
materia-repo/secrets/vault.age
materia-repo/MANIFEST.toml
```

Push your git repository and logon to `ivy` for the next steps.

## Validate your repository with materia plan

Materia uses a plan-execute system for managing nodes. We can generate a plan ahead of time for validation purposes.

These steps will assume your git repository is at `github.com/user/materia-repo` and is **public**. If you need to use a private repository, look at the [git configuration settings](reference/materia-config-git.5.md)

### Configure materia's source URL

Materia needs to know where your repository is. This can be done in a config file, but we'll just use an environmental variable

`export MATERIA_SOURCE_URL=git://git@github.com:user/materia-repo"`

### Update known_hosts (OPTIONAL)

This is optional if you're using git over HTTP or you've already set this up on your host (i.e. the root user can clone your repository without any input).

Make sure your Git forge is in your `known_hosts` file for SSH access:

`ssh-keyscan github.com >> ~/.ssh/known_hosts`

### Run materia plan

Use the `plan` command to see what materia will do when you run it. The output should look something like this:

```
$ materia plan
Plan:
1. Installing component caddy
2. Templating container resource caddy/caddy.container
2. Installing volume resource caddy/caddy-config.volume
3. Installing volume resource caddy/caddy-data.volume
4. Templating file resource caddy/Caddyfile
3. Reloading systemd units
4. Starting service caddy/caddy.service

$
```

The output is deterministic and will make sure that your repository is valid. If there's any missing secrets or other information not available to the host it will fail.

## Update the host

Finally, use the update command to update `ivy` to the desired state:

```
$ materia update
Plan:
1. Installing component caddy
2. Templating container resource caddy/caddy.container
2. Installing volume resource caddy/caddy-config.volume
3. Installing volume resource caddy/caddy-data.volume
4. Templating file resource caddy/Caddyfile
3. Reloading systemd units
4. Starting service caddy/caddy.service

$ ls /etc/containers/systemd/
caddy
$ ls -a /etc/containers/systemd/caddy
. .. caddy.container caddy-config.volume caddy-data.volume .materia_managed
$ ls -a /var/lib/materia/components/caddy
. .. .component_version conf/
$ systemctl is-active caddy.service
active
$
```


