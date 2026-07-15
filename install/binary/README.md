# Binary Installation

These files assume you've installed the `materia` binary to `/usr/bin/materia`. Adjust the `ExecStart=` properties to reflect the actual location, especially if running rootless.

## Update workflow files

Runs `materia update` every five minutes:

- `materia-update.service` -> `/etc/systemd/system/materia-update.service`
- `materia-update.timer` -> `/etc/systemd/system/materia-update.timer`
- `sysconfig-materia`  -> `/etc/sysconfig/materia`


## Server workflow files

Runs `materia server` configured to update every ten minutes

- `materia-server.service` -> `/etc/systemd/system/materia-server.service`
- `sysconfig-materia-server`  -> `/etc/sysconfig/materia-server`
