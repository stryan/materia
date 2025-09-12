# Changelog

Best effort list of major changes and bugfixes

## Unreleased features
- Resources now use their full relative filepaths as their names. If you have a resource in a folder e.g. `/var/lib/materia/source/components/hello/inner/foo.txt` it's name would be `inner/foo.txt` not `foo.txt`
- Resource clean-up supports custom network/pod/volume names

## 0.2.0
- Podman secrets integration in component manifests
- Resource cleanup upon quadlet removal

## 0.1.0
- Initial release
