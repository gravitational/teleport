## Docker

This directory contains Docker-based flow to run Teleport clusters locally
for testing & development purposes.

### Building 

First, you need to build `teleport:latest` Docker image. This image is built
automatically when you type `make` BUT...

But you have to build the base image first, by running `make -C build.assets`
from `$GOPATH/github.com/gravitational/teleport` (repository base dir).

### Starting 

Type:

```bash
$ make
```

This will start two Teleport clusters:

* Single-node cluster `one`, accessible now on https://localhost:3080
* Three-node cluster `two`, accessible now on https://localhost:5080

### Stopping

Type:

```bash
$ make stop
```

### Configuration

Look at the [Makefile](Makefile): the containers are started with their 
`/var/lib/teleport` mounted to `data/one` or `data/two` on a host. 

The configuration is passed via YAML files located in `/teleport/docker/xxx.yaml` 
inside each container.

The cluster data is preserved between restarts, so you can link these two
clusters (make them "trusted") by placing certificates within `data` and 
updating the config files.

### Using TCTL

To add users to any of the clusters, you have to "enter" into the running
containers of their auth servers and use `tctl` there.

For cluster "one":

```bash
$ make enter-one
```

and then you'll find yourself inside a container where `teleport` auth daemon
is running, try `ps -ef` for example and you'll get something like this:

```bash
container(one) /teleport: ps -ef
UID        PID  PPID  C STIME TTY          TIME CMD
root         1     0 40 06:04 ?        00:00:06 build/teleport start -c /teleport/docker/one.yaml
root        13     0  0 06:04 ?        00:00:00 /bin/bash
root        19    13  0 06:04 ?        00:00:00 ps -ef
```

For cluster "two":

```bash
$ make enter-two
```

... and then you can use stuff like `tctl users add`, etc. Make sure to pass 
the YAML file to `tctl` via `-c` flag.

### Interactive Usage

Also you can start an empty container from which you can manually invoke `teleport start`. 
This is similar to launching an empty Linux VM with a Teleport binary.

Do:

```bash
$ make shell
```

if you get "network already exists" error, do `make stop` first.

