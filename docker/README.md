## Docker

This directory contains Docker-based flow to run Teleport clusters locally
for testing & development purposes.

### Building

First, you need to build `teleport:latest` Docker image.

Run the following commands from `$GOPATH/github.com/gravitational/teleport` (repository base dir):

```bash
$ make docker
$ cd docker
$ make build
```

### Starting

```bash
$ make up
```

This will start two Teleport clusters:

* Single-node cluster `one`, accessible now on https://localhost:3080
* Three-node cluster `two`, accessible now on https://localhost:5080

### Stopping

```bash
$ make down
```

### SSH

SSH container needs User CA authorities exported:

```bash
$ make export-certs
```

### Configuration

Look at the [Makefile](Makefile): the containers are started with their
`/var/lib/teleport` mounted to `data/one` or `data/two` on a host.

The configuration is passed via YAML files located in `/teleport/docker/xxx.yaml`
inside each container.

Since the cluster data is preserved between restarts, so you can edit the configuration
and restart if you want to change configuration changes.

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

### Trusted Clusters with Resources

1. Update `two-role.yaml` and replace `username_goes_here` with your username.
1. Create a `Role` and `TrustedCluster` resource on Cluster Two.

    ```bash
    make enter-two
    tctl -c /root/go/src/github.com/gravitational/teleport/docker/two-auth.yaml create -f docker/two-role-admin.yaml
    tctl -c /root/go/src/github.com/gravitational/teleport/docker/two-auth.yaml create -f docker/two-tc.yaml
    ```

### Ansible

To setup Ansible:

1. Follow steps in Trusted Cluster section to setup Trusted Clusters.
1. Use `tctl` to issue create user command and follow link on screen to create user.

    ```bash
    tctl users add {username} root
    ```
1. Configure Ansible.

    ```bash
    # add two-node to ansible hosts file
    echo "172.10.1.2:3022" >> /etc/ansible/hosts

    # setup ssh_args that ansible will use to access trusted cluster nodes
    sed -i '/ssh_args = -o ControlMaster=auto -o ControlPersist=60s/assh_args = -o "ProxyCommand ssh -p 3023 one -s proxy:%h:%p@two"' /etc/ansible/ansible.cfg

    # use scp over sftp
    sed -i '/scp_if_ssh/s/^#//g' /etc/ansible/ansible.cfg
    ```

1. Start and load OpenSSH agent with keys.

    ```bash
    # create directory for ssh config
    mkdir ~/.ssh && chmod 700 ~/.ssh

    # start ssh-agent
    eval `ssh-agent`

    # log in with the user created before
    tsh --proxy=localhost --user=rjones login

    # load keys into ssh-agent
    tsh --proxy=localhost --user=rjones agent --load
    ```

1. Verify Ansible works:

    ```bash
    $ ansible all -m ping
    172.10.1.2 | success >> {
        "changed": false,
        "ping": "pong"
    }
    ```

1. Run an simple playbook:

    ```bash
    # cd to directory that contains playbook
    cd /root/go/src/github.com/gravitational/teleport/docker/ansible

    # run playbook
    ansible-playbook playbook.yaml
    ```

### Interactive Usage

Also you can start an empty container from which you can manually invoke `teleport start`.
This is similar to launching an empty Linux VM with a Teleport binary.

To get shell inside the same "one" (single-node cluster) container without
Teleport running:

```bash
$ make shell
```

NOTE: If you get "network already exists" error, do `make stop` first.

Once inside, you'll get the same `/var/lib/teleport` as "one", so you
can start (and even build) `teleport` daemon manually. This container also
comes with a fully configured `screen` so you can treat it as a real VM.

