# Install Methods

Included here are systemd unit files for running Materia. For more tips on running Materia, see the official documentation site [here](https://primamateria.systems/documentation/latest/running.html)

## Containerized options

For running materia using the provided [container](https://github.com/stryan/materia/pkgs/container/materia).

- [Rootful](./root/): the most well-tested and supported method, runs `materia update` on a regular basis using systemd timers
- [Rootful Server-mode](./root-server/): Runs `materia server` as a service in the background, configured to update the system every 15 minutes
- [Rootless](./rootless/): Runs `materia update` on a regular basis but is designed for as a non-root user.

## Binary

Systemd unit files for running just the binary are available [here](./binary/).

These should work for both root and rootful setups, though be sure to update the `ExecStart=` path to match where you installed the binary.
