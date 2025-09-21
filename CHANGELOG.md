# Changelog

Best effort list of major changes and bugfixes

## Unreleased
- Configuring secrets in repository manifests is removed
- Volume File Resources are removed (they never really worked)
- Materia secrets are renamed as Component Attributes (I got tired of writing "materia secrets" to differentiate from Podman secrets)

## 0.2.1
- fixed bug in configuration precedence rules: should now work as expected (i.e. CLI overrides Env overrides Config)

## 0.2.0
- Podman secrets integration in component manifests
- Resource cleanup upon quadlet removal

## 0.1.0
- Initial release
