# Teleport

Teleport is a SSH infrastructure for clusters of Linux servers. Teleport extends 
traditional SSH with the following capabilities:

* Provides coordinated and secure access to multiple Linux clusters by multiple teams 
  with different permissions.
* Enforces cluster-specific security policies.
* Includes session record/replay and keeps audit logs.

It also contains a few nice conveniences like built-in command multiplexing, web-based
administration and more. `Teleport` is a standalone executable. It has one external 
dependency: [Etcd](https://coreos.com/etcd/)

## Developer Docs

Take a look at [Developer API](docs/api.md)

## Overview

![Overview](docs/img/teleport.png)

A Teleport daemon needs to be running on every server in a cluster. Each instance 
assumes one of these roles:

* Auth server
* SSH server
* Proxy

**Auth server**

Auth server is connected to Etcd backend or embedded bolt database and acts as:

* Teleport Auth Server acts as Authentication and Authorization server,
* SSH host and user certificate authority
* Stores audit logs and access records and is the only stateful component in the system.


Read more about SSH authorities in this [intro article](https://www.digitalocean.com/community/tutorials/how-to-create-an-ssh-ca-to-validate-hosts-and-clients-with-ubuntu)

**Note:** Auth server does not itself provide any support for interactive sessions and remote command execution

**SSH server**

Teleport SSH server is a simple stateless server written in Go that only supports SSH user certificates as authentication method, generates structured events and supports interactive collaborative sessions

**Web access portal**

Web access portal is stateless too, it connects to the auth server
and provides SSH access, view of the logs and key management interfaces.

## Installation

Teleport is currently a private project and should be cloned from
the repository.

**Prerequisites**

* `go >= 1.4.2`
* `etcd >= v2.0.10`

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

This should install the binaries, check that the binaries are installed.

```shell
ls ${GOPATH}/bin/tctl ${GOPATH}/bin/teleport
```

## Auth Server

```shell
# create the directory where auth server will keep it's local state
mkdir -p /var/lib/teleport-auth

# start teleport binary with auth role enabled
teleport -auth\
         -authBackend=etcd\
         -authBackendConfig='{"nodes": ["http://127.0.0.1:4001"], "key": "/teleport"}'\
         -authDomain=example.com\
         -authSSHAddr=tcp://0.0.0.0:32000\
         -log=console\
         -logSeverity=INFO\
         -dataDir=/var/lib/teleport-auth\
         -fqdn=auth.example.com
```

**Note:** `authSSHAddr` sets up a special-purpose SSH-powered API endpoint that the auth server exposes for nodes and web portals to check access.

## SSH Server

On the first connection attempt SSH nodes will attempt to connect to the auth server and register itself.
Get a one-time security token from the auth server for a SSH node to connect to the server:


**Step 1. Get the token**

This command will generate a one time secure token allowing server `node1.example.com` to register with the server within the next 120 seconds.

```shell
tctl token generate -fqdn=node1.example.com
```

**Step 2. Start the SSH node**

Pass the token from the step 1 and pass it to the teleport SSH node on start:

```shell
# create a directory for the local node state
mkdir -p /var/lib/teleport-node

# start the server
TELEPORT_SSH_TOKEN=<token here> teleport -ssh\
             -log=console\
             -logSeverity=INFO\
             -dataDir=/var/lib/teleport-node\
             -fqdn=node1.example.com\
             -authServer=tcp://auth.example.com:32000
             -sshAddr=tcp://0.0.0.0:33000
```

SSH node will use the token to connect to the auth server and provision the signed SSH host certificate and private key.
The subsequent starts/restart will use the host certificates to authenticate with the Auth server.

## Web Server

Control panel node is optional, and is only needed if you want to access the cluster via web interface.

```shell
    # create a directory for the local node state
    mkdir -p /var/lib/teleport-cp

	teleport -cp\
             -cpDomain=example.com\
             -log=console\
             -logSeverity=INFO\
             -dataDir=/var/lib/teleport-cp\
             -fqdn=cp.gravitational.io\
             -authServer=tcp://auth.gravitational.io:32000
```

**Note:** Unlike SSH node, CP node does not need to use any provisioning step, as it's just a web interface using SSH auth server APIs.

**Step 1. Get the token**

This command will generate a one time secure token allowing server `node1.example.com` to register with the server within the next 120 seconds.

```shell
tctl token generate -fqdn=node1.example.com
```

**Step 2. Start the SSH node**

Pass the token from the step 1 and pass it to the teleport SSH node on start:

```shell
# create a directory for the local node state
mkdir -p /var/lib/teleport-node

# start the server
TELEPORT_SSH_TOKEN=<token here> teleport -ssh\
             -log=console\
             -logSeverity=INFO\
             -dataDir=/var/lib/teleport-node\
             -fqdn=node1.example.com\
             -authServer=tcp://auth.example.com:32000
             -sshAddr=tcp://0.0.0.0:33000
```

SSH node will use the token to connect to the auth server and provision the signed SSH host certificate and private key.
The subsequent starts/restart will use the host certificates to authenticate with the Auth server.

### SSH access

Connecting to Teleport is like connecting to any other SSH server, except that it
does not support password and host based auth, and only works with keys signed by the authority.

**Sign the SSH key**

If you don't have they key yet:

```shell
ssh-keygen -t rsa -b 4096 -C "your_email@example.com"
```

To sign the key, you can use the tctl command on the auth server:

```shell
tctl user upsert_key -user=user -keyid=user-key1 -key=user.pub
```

Teleport user CA (certificate authority) will sign the key and returned the signed certificate.
You can place it near the private and public keys in file `user-cert.pub` and use it to connect to the server.

**Add the keys to agent**

SSH agent is a little program that holds the keys in memory and authenticates on your behalf.
Check if ssh agent is running:

```shell
pidof ssh-agent
```

If the command above returns nothing, start the agent:

```shell
eval $(ssh-agent)
```

Add keys to the agent:

```shell
ssh-add /<path-to-key>/user
```

Check if the keys are loaded

```shell
ssh-add -l
2048 8f:c2:cf:85:c4:02:7a:f9:73:ae:c7:62:ae:c0:36:04 rsa w/o comment (RSA)
2048 8f:c2:cf:85:c4:02:7a:f9:73:ae:c7:62:ae:c0:36:04 rsa w/o comment (RSA-CERT)
```

Log in:

```ssh
ssh -p 33000 node1.example.com
```

#### Trusting Host CA

You may have noticed the following warning when connecting to the host:

```shell
The authenticity of host '[node1.gravitational.io]:33000 ([127.0.0.1]:33000)' can't be established.
RSA key fingerprint is 9d:ff:8b:aa:b5:af:70:33:90:a8:1c:1e:85:af:02:a6.
Are you sure you want to continue connecting (yes/no)
```

This warning means that your client have not seen this host before and is not sure if it's trusted or not.
It will be displayed each time you log in to the server that was not seen before.

This is quite annoying to deal with, and thankfully, there's a solution for it.
All host keys used by teleport are signed by the host authority. Instead of validating the keys,
you can validate the authority.

**Get the host CA certificate**

```shell
# from the auth server:
tctl hostca pubkey
```

Copy the certificate and write this line to your home folder's  `.ssh/known_hosts`:

```shell
@cert-authority *.example.com <certificate-key-here>
```

This line will tell the SSH client to trust all hosts whose public keys are signed by this authority.

### Web access

Teleport allows to access the cluster via optional web portal handled by CP server.

**Create a password**

Grant user web access using auth server user api

```shell
 tctl user set_pass -user=alex -pass=pwd123
```

Now users can log in using their usernames and passwords into the portal



