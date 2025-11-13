# Changelog

Best effort list of major changes and bugfixes

## Upcoming

## 0.4.2

- feat: restart containers and pods when resource updated by default
- Source types can now be specified with the `source.kind` option. The current `git://` or `file://` methods should work as before, see the `materia-source` reference page for details.
- bugfix: fix nosync function
- bugfix: use private key in .ssh when one isn't specified
- feat: support all unit types


## 0.4.1
- `materia` now does a dry-run of Quadlets update when running `systemctl daemon-reload` to prevent installing bad Quadlet files over working ones
- Support `.build` and `.image` quadlets
- bugfix: use triggered actions versus reload/restart map
- bugfix: correct when host reloads

## 0.4.0
- `autoUpdate` is now a macro as well as a snippet. The snippet will be deprecated Eventually (tm)
- Multiple Attributes engines can be configured; Materia will query all of them and merge the results in an unspecified order.
    - Related, the `attributes` configuration value now only forces the use of a specific engine and is no longer required.
- Materia manifests now support overriding component manifests for a given host
- `materia server` and `materia agent` now exist to provide a more classical GitOps experience
- Repository Manifests now support `Remote` components; components downloaded from a remote git repository or other location
- `Plan` now supports setting the output format. Adds support for JSON output.
- Env variable settings now correctly use `MATERIA_AGE__KEYFILE` format for attributes/source/other sub configs
- New `MIGRATE_VOLUMES` config option to enable volume migration on quadlet update: if a `.volume` quadlet is updated materia will:
    1. Stop services for the component
    2. Dump the existing volume to a tarball
    3. Delete the existing volume
    4. Restart the updated service to create the new volume
    5. Import the old volume tarball into the new volume

## 0.3.0
- Materia secrets are renamed as Component Attributes in order to better differentiate them from podman secrets.
- SOPS support as secrets backend
- Configuring attributes in repository manifests is removed, at least for now. The benefits of doing so were minimal and it greatly simplifies configuration.
- more flexible testing harness
- All manifest keys are now CamelCased to match systemd style. All config keys are now lowercase and snake_case to be more readable.
- Volume File Resources are removed (they never really worked)
- Resources now use their full relative filepaths as their names. If you have a resource in a folder e.g. `/var/lib/materia/source/components/hello/inner/foo.txt` it's name would be `inner/foo.txt` not `foo.txt`
- Resource clean-up supports custom network/pod/volume names

## 0.2.1
- fixed bug in configuration precedence rules: should now work as expected (i.e. CLI overrides Env overrides Config)

## 0.2.0
- Podman secrets integration in component manifests
- Resource cleanup upon quadlet removal

## 0.1.0
- Initial release
