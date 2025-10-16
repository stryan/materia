# Changelog

Best effort list of major changes and bugfixes

## Upcoming

## Unreleased
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
