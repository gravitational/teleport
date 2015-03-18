# Teleport

Proxying SSH server with dynamic backend and ACL support

## Rationale

The major pain points for any operations is the absense of clear and unified access to the server.

The question operations have is:

* How and where do I log in and where's the password?

The question users allowing access have:

* How do I grant/revoke access to anyone?
* How do I grant temporary access?
* How do I track the actions on the servers?
* What is my entrypoint?

## Implementation

The goal of this project is to define a simple SSH infrastrucure that satisfies the following requirements:

* It is ridiculously easy to setup. Ideally it should be zero-configuration for users.
* Embraces the notion of "bastion", where the entry point server grants access to the cluster.
* Provides workflow for granting/revoking access to users and groups.
* Provides infrastructure for logging SSH activity in the cluster in structured format
* Supports dynamic configuration backend.

### Implementation thoughts and open questions:

* Choice of a dynamic backend (Etcd?)
* Choice of auth (PKI or OpenSSH CA?)
* SSH server implementation (OpenSSH, go ssh?)


## Proposed design

![teleport overview](/doc/img/TeleportOverview.png?raw=true "Teleport Overview")

* Use Etcd for configuration, discovery, presence and key store
* Each server has ssh server heartbeating into the cluster
* Use OpenSSH key infractrucure for authorizing/rejecting access
* Use Go SSH server to multiplex connections, talk to Etcd for checking access by public keys
* Implement a service and a CLI for managing/revoking and signing certificates
* Use structured logging in syslog and json format and plug it into ES cluster


## Instalation and run

## Setting up certificates

Running teleport requires some set up of the certificates at the moment.

1. Create a server CA

This CA will be used to sign host certificates

```shell
ssh-keygen -f server_ca
```

2. Use server CA to sign the host key on your machine

```shell
# signs server cert with authority key
ssh-keygen -s server_ca -I host_auth_server -h -n auth.example.com -V +52w /etc/ssh/ssh_host_rsa_key.pub
```

3. Create a user CA

This authority will be used to sign user's public keys:

```shell
# generate key pair for authority signing user keys
ssh-keygen -f users_ca
```

4. Generate a user public/private key pair

This keypair will be used to access teleport server

```shell
ssh-keygen -f <put-user-name-here>
```

5. Use user CA to sign the user key

```shell
ssh-keygen -s users_ca -I user_username -n <put-user-name-here> -V +52w id_rsa.pub
```

Read on the guide here for more information: 

https://www.digitalocean.com/community/tutorials/how-to-create-an-ssh-ca-to-validate-hosts-and-clients-with-ubuntu

## Running the teleport server

```shell
teleport -host=localhost -port=2022 -hostPrivateKey=/etc/ssh/ssh_host_rsa_key -caPublicKey=./path-to/users_ca.pub
```

## Connecting to teleport server

1. Start SSH agent

```shell
eval `ssh-agent`
```

2. Add the user key created to the agent (see the previous section)

```shell
ssh-add /path-to-user-private-and-public-key-directory
```

Make sure the identity has been loaded by running

```shell
ssh-add -l
```

3. Instruct ssh client to turn on agent forwarding

edit file `~./ssh/config` and add the following lines:

```
Host *
     ForwardAgent yes
```

These lines allow ssh-client to use running ssh-agent for authorization with ssh server


4. If everything is correct, the following command will succeed:

```
ssh -p 2022 localhost
```

## Using teleport features

## TUN subsystem

Tun subsystem uses one teleport server to access another teleport server using client public key's without extra hassle of key management.

```
ssh -p 2022 localhost -s tun:127.0.0.1:2022
```


## MUX subsystem

MUX subsystem instructs teleport to connect to remote servers, execute the desired commands, collect return back the responses to the client:

```shell
# instructs teleport to run command 'ls -l' on 127.0.0.1 port 2022  and localhost port 2022
ssh -p 2022 localhost -s mux:127.0.0.1:2022,localhost:2022/ls -l
```




