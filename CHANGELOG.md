# Changelog

Best effort list of major changes and bugfixes

## Upcoming
- Materia secrets are renamed as Component Attributes (I got tired of writing "materia secrets" to differentiate from Podman secrets)

## Unreleased
- SOPS support as secrets backend
- All manifest keys are now CamelCased to match systemd style
- Configuring secrets in repository manifests is removed
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
