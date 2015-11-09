# Teleport

Teleport is a SSH infrastructure for clusters of Linux servers. Teleport extends 
traditional SSH with the following capabilities:

* Provides coordinated and secure access to multiple Linux clusters by multiple teams 
  with different permissions.
* Enforces cluster-specific security policies.
* Includes session record/replay and keeps audit logs.

It also contains a few nice conveniences like built-in command multiplexing, web-based
administration and more. `Teleport` is a standalone executable.

Teleport uses [Etcd](https://coreos.com/etcd/) in HA mode or [Boltdb](https://github.com/boltdb/bolt) in standalone mode.

## Status

**Teleport is not ready to be used in production yet**

We are currently fixing outstanding security issues, and working on hardening.

## Design document

Take a look at [Teleport design document](https://docs.google.com/a/gravitational.io/document/d/10-DjtvKFjsiPHcMDArHtjvepdQg5iZUWSafAF03OBbE/edit?usp=sharing)

## Developer Docs

Take a look at [Developer API](docs/api.md)

## Overview

![Overview](docs/img/teleport.png)

Teleport system consists of several independent parts that can be set up in various combinations:

**Teleport Auth**

Auth server acts as Authentication and Authorization server, SSH host and user certificate authority, stores audit logs and access
records and is the only stateful component in the system.

*Note* Read more about SSH authorities in this [intro article](https://www.digitalocean.com/community/tutorials/how-to-create-an-ssh-ca-to-validate-hosts-and-clients-with-ubuntu)
*Note* Auth server does not itself provide any support for interactive sessions and remote command execution

**Teleport SSH**

Teleport SSH server is a simple stateless server written in Go that only supports SSH user certificates as authentication method,
generates structured events and supports interactive collaborative sessions.

**Teleport Proxy**

Teleport Proxy is a stateless SSH proxy that implements 2-factor web authentication and proxies traffic to the remote SSH nodes.

## Installation

Teleport is open source, however it   and should be cloned from the repository.

**Prerequisites**

* `go >= 1.4.2`
* `etcd >= v2.0.10` (in case of HA mode)

**Clone the latest master**

```shell
mkdir -p $(GOPATH)/src/github/gravitational
cd $(GOPATH)/src/github/gravitational
git clone git@github.com:gravitational/teleport.git
```

**Compile**

```shell
make install
```

This should install `teleport` and `tctl` binaries, check that the binaries are installed.

```shell
ls ${GOPATH}/bin/tctl ${GOPATH}/bin/teleport
```

## Quickstart

```shell
# create the directory where auth server will keep it's local state
mkdir -p /var/lib/teleport
# make sure it is not owned by root
chown <<USER>>:<<GROUP>> /var/lib/teleport

# start teleport in embedded mode
make run-embedded
```

**Note:** `run-embedded` executes teleport with configuration file in `examples/embedded.yaml` check it out for more details

### Web access via proxy

Teleport allows to access the cluster via web portal. The web portal is guarded by 2-factor authentication. Here's how to log in:


* Create a user entry for yourself:

```shell
tctl user set-pass --user=<user> --pass=<pass>
```

**Important:** Username and password are not enough to log in into teleport, for second factor it uses HOTP tokens.
Tool generated QR code for you too, and placed it in the current working directory. Follow next steps to set up your phone to use QR key:

* Set up Google Authenticator app on your phone (available for free for Android and iPhone)

Check out QR.png file that was written to the local directory and scan QR code. Follow next step to login:

* Open a browser: http://localhost:33007/web/login and enter username, password and QR code

**Note:** If you failed to log in for the first time, try to refresh the token. Teleport will try to sync up your phone and token on the next attempt.


### SSH access via proxy

#### OpenSSH

To use OpenSSH client with Teleport you need to run Teleport ssh agent on your local machine.

1. First, start the agent
  
  ```shell
  tctl agent start --agent-addr="unix:///tmp/teleport.agent.sock"
  ```
2. Then your need to login your agent using your credentials
  
  ```shell
  tctl agent login --proxy-addr=PROXY-ADDR --ttl=10h
  ```
  where PROXY-ADDR - address of the remote Teleport proxy, ttl - time you want to be logged in (max 30 Hours).
  tctl will ask you your username, password and 2nd token.
3. Modify default agent address
  
  ```shell
  SSH_AUTH_SOCK=/tmp/teleport.agent.sock; export SSH_AUTH_SOCK;
  ```
4. To enable connecting via proxy on the OpenSSH client add ProxyCommand to ~/.ssh/config file. For example:
  
  ```
  Host node1.gravitational.io
    ProxyCommand  ssh -p {proxyport} %r@proxy.gravitational.io -s proxy:%h:%p
  ```
5. Then you can connect to your ssh nodes as usual:
  
  ```shell
  ssh -p {nodeport} user@node1.gravitational.io
  ```

#### Ansible

By default Ansible use OpenSSH client. To make Ansible work with Teleport you need:
* config your OpenSSH client
* enable scp mode in the Ansible config file(default /etc/ansible/ansible.cfg):
  
  ```
  scp_if_ssh = True
  ```
