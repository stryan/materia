# Running Materia

There are three supported methods of running materia:

1. [Rootful Containerized](#containerized)
2. [Bare metal](#bare-metal)
3. [Rootless Containerized](#rootless-containerized)


## Rootful Containerized

In this method you are running Materia using a Podman container (or Quadlet) run by the root user on the host.

Rootful Containerized is the recommended way of running Materia. It is the most well tested pathway and the default assumption when designing features.

No special installation notes are needed for this method as the Materia defaults cover it. An example quadlet for running Rootful Containerized Materia can be found at `install/materia@.container` in the [source repository](https://github.com/stryan/materia/blob/master/install/materia%40.container).

##  Bare metal

In this method you are running the Materia binary directly on the target host.

This method is less tested than the Rootful Containerized method, but is still relatively well tested.

You can run Materia as either root or another user with this method: Materia will automatically use the `XDG_DIR` settings for it's directories if run as a normal user, as well as the users podman and systemd sockets. See the reference section for what directories those are.

Note that Materia will *not* make any adjustments to your user systemd settings by default. You may need to set configuration like `loginctl enable-linger` yourself.


## Rootless Containerized

In this method you are running Materia using a Podman container (or Quadlet) run by a non-root user on the host.

Rootless containerized is (currently) not recommended for production use. It is not tested during development and while it should work you may run into unexpected issues. This will be fixed before 1.0.

Materia runs as root within the container, so some adjustments need to be made for volume mounts and macros to work:

You should mount your user podman socket and systemd sockets as root's sockets within the container i.e `Volume=/run/user/1000/podman/podman.sock:/run/podman/podman.sock` not `Volume=/run/user/1000/podman/podman.sock:/run/podman/podman.sock`.

Some macros like `m_dataDir` reference materia path's directly. This means, when the rootless container is setup as described, it will template the root path into templated files instead of the correct host path. In order to fix this you need to do one of the following:

1. Bind mount the correct user locations into the container and manually configure materia to use them. I.e. for the quadlets directory you would bind mount `-v /home/user/.config/containers/systemd:/home/user/.config/containers/systemd` and also set the config option `-e MATERIA_QUADLET_DIR=/home/user/.config/containers/systemd`.

2. Manually specify the resulting host directories with the `materia.executor.materia/scripts/quadlet/service_dir` configuration options.

3. Use the `materia.rootless`/`MATERIA_ROOTLESS=true` config option. This is an experimental configuration option to make setting this up a little easier. You use it by bind mounting the user's directories (e.g `/home/user/.local/share/materia/`) as the normal rootful locations (e.g. `/var/lib/materia`). Materia will then try to auto-discover it's own Podman container during start up and extract the correct host mounts, meaning you do not have to set the directory locations yourself. This is an automated version of option 2.

An example quadlet for running Rootless Containerized Materia can be found at `install/materia@.container` in the [source repository](https://github.com/stryan/materia/blob/master/install/materia-rootless.container).

