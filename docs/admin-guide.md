# Admin Guide

## Installation

### Installing from Source

Gravitational Teleport is written in Go language. It requires Golang v1.5 or newer. 
If you have Go already installed, type:

```bash
> git clone https://github.com/gravitational/teleport && cd teleport
> make 
```

If you do not have Go but you have Docker installed and running, you can build Teleport
this way:

```bash
> git clone https://github.com/gravitational/teleport
> make -C build.assets
```

### Installing from Binaries

You can download binaries from [Github](https://github.com/gravitational/teleport/releases). 

## Running

Teleport supports only a handful of commands

|Command     | Description
|------------|-------------------------------------------------------
|start       | Starts the Teleport daemon.
|configure   | Dumps a sample configuration file in YAML format into standard output.
|version     | Shows the Teleport version.
|status      | Shows the status of a Teleport connection. This command is only available from inside of an active SSH seession.
|help        | Shows help.

When experimenting you can quickly start `teleport` with verbose logging by typing 
`teleport start -d`.

In production, we recommend starting teleport daemon via init system like `systemd`. 
Here's an example systemd unit file:

```
[Unit]
Description=Teleport SSH Service
After=network.target 

[Service]
Type=simple
Restart=always
ExecStart=/usr/bin/teleport --config=/etc/teleport.yaml start

[Install]
WantedBy=multi-user.target
```

## Configuration

`teleport` daemon is configured via a configuration file but for most common use cases you 
can only use command line flags.

### Configuration Flags

hello

### Configuration File

Teleport uses YAML file format for its configuration file. By default it is stored in 
`/etc/teleport.yaml`

**WARNING:** When editing YAML configuration, please pay attention to how your editor 
             handles white space. YAML requires consistent handling of tab keys.

Below is the sample configuration file:

```yaml
# By default, this file should be stored in /etc/teleport.yaml

# This section of the confniguration file applies to all teleport
# services.
teleport:
  # nodename allows to assign an alternative name this node can be reached by.
  # by default it's equal to hostname
  nodename: graviton

  # one-time invitation token used to join a cluster. it is not used on 
  # subsequent starts
  auth_token: xxxx-token-xxxx

  # list of auth servers in a cluster. you will have more than one auth server
  # if you configure teleport auth to run in HA configuration
  auth_servers: 10.1.0.5:3025, 10.1.0.6:3025

  # Teleport throttles all connections to avoid abuse. These settings allow
  # you to adjust the default limits
  connection_limits:
    max_connections: 1000
    max_users: 250

  # Logging configuration. Possible output values are 'stdout', 'stderr' and 
  # 'syslog'. Possible severity values are INFO, WARN and ERROR (default).
  log:
    output: stderr
    severity: ERROR

  # Type of storage used for keys. You need to configure this to use etcd
  # backend if you want to run Teleport in HA configuration.
  storage:
    type: bolt
    data_dir: /var/lib/teleport

# This section configures 'auth service':
auth_service:
  enabled: yes
  listen_addr: 127.0.0.1:3025

# This section configures 'node service':
ssh_service:
  enabled: yes
  listen_addr: 127.0.0.1:3022
  labels:
    role: master
    type: postgres
  commands:
  - name: hostname
    command: [/usr/bin/hostname]
    period: 1m0s
  - name: arch
    command: [/usr/bin/uname, -p]
    period: 1h0m0s

# This section configures the 'proxy servie'
proxy_service:
  enabled: yes
  listen_addr: 127.0.0.1:3023
  web_listen_addr: 127.0.0.1:3080
  https_key_file: /etc/teleport/teleport.key
  https_cert_file: /etc/teleport/teleport.crt

```

## Adding users to the cluster


## Controlling access

At the moment `teleport` does not have a command for modifying an existing user record.
The only recipe to update user mappings or reset user password is to remove the account
and re-create it. 

The user will have to re-initialize Google Authenticator on their phone.

## Adding nodes to the cluster



