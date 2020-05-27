# Teleport Pagerduty Plugin Setup

This package provides a Teleport ↔  Pagerduty integration that allows you to treat Teleport access and permission requests as Pagerduty incidents — and notify the appropriate team, and approve or deny the requests via Pagerduty special action.

## Prerequisites
This guide assumes you have

* Teleport Enterprise 4.2.8 or newer with admin permissions and access to `tctl`.
* Pagerduty account already set, with access to creating a new API token. 
* A Node to run the plugin, we recommend running it alongside the Teleport Proxy for convenience. 

#### Create User and Role for access. 
Log into Teleport Authentication Server, this is where you normally run `tctl`. Don't change the username and the role name, it should be `access-plugin` for the plugin to work correctly.

_Note: if you're using other plugins, you might want to create different users and roles for different plugins_.

```bash
$ cat > rscs.yaml <<EOF
kind: user
metadata:
  name: access-plugin
spec:
  roles: ['access-plugin']
version: v2
---
kind: role
metadata:
  name: access-plugin
spec:
  allow:
    rules:
      - resources: ['access_request']
        verbs: ['list','read','update']
    # teleport currently refuses to issue certs for a user with 0 logins,
    # this restriction may be lifted in future versions.
    logins: ['access-plugin']
version: v3
EOF

# ...
$ tctl create -f rscs.yaml
```

#### Export access-plugin Certificate
Teleport Plugin use the `access-plugin` role and user to perform the approval. We export the identity files, using [`tctl auth sign`](https://gravitational.com/teleport/docs/cli-docs/#tctl-auth-sign).

```bash
$ tctl auth sign --format=tls --user=access-plugin --out=auth --ttl=8760h
# ...
```

The above sequence should result in three PEM encoded files being generated: auth.crt, auth.key, and auth.cas (certificate, private key, and CA certs respectively).  We'll reference these later when [configuring Teleport-Plugins](#configuration-file).

!!! note "Certificate Lifetime"
     By default, tctl auth sign produces certificates with a relatively short lifetime. For production deployments, the `--ttl` flag can be used to ensure a more practical certificate lifetime. `--ttl=8760h` exports a 1 year token

### Setting up Pagerduty API key

In your Pagerduty dashboard, go to **Configuration → API Access → Create New API Key**, add a key description, and save the key. We'll use the key in the plugin config file later.

**Create Pager Duty API Key**
![Create a service account](/img/enterprise/plugins/pagerduty/pagerduty-api-key.png)

**Create Service Account**
![Create a service account](/img/enterprise/plugins/pagerduty/create-new-service-pd.png) 


## Downloading and installing the plugin

The recommended way to run Teleport Pagerduty plugin is by downloading the release version and installing it: 

```bash
$ wget https://get.gravitational.com/teleport-access-pagerduty-v{{ teleport.plugin.version }}-linux-amd64-bin.tar.gz
$ tar -xzf teleport-access-pagerduty-v{{ teleport.plugin.version }}-linux-amd64-bin.tar.gz
$ cd teleport-access-pagerduty/
$ ./install
$ which teleport-pagerduty
/usr/local/bin/teleport-pagerduty
```

### Config file
Teleport Pagerduty plugin has its own configuration file in TOML format. Before starting the plugin for the first time, you'll need to generate and edit that config file. 

```bash
teleport-pagerduty configure > /etc/teleport-pagerduty.toml
```

#### Editing the config file
After generating the config, edit it as follows: 

```yaml
# lol
{!examples/resources/plugins/teleport-pagerduty.toml!}
```

### Testing the Plugin

With the config above, you should be able to run the plugin invoking 
`teleport-pagerduty start -d`. The will provide some debug information to make sure
the bot can connect to Pagerduty. 

```bash
$ teleport-pagerduty start -d
DEBU   DEBUG logging enabled logrus/exported.go:117
INFO   Starting Teleport Access PagerDuty extension 0.1.0-dev.1: pagerduty/main.go:124
DEBU   Checking Teleport server version pagerduty/main.go:226
DEBU   Starting a request watcher... pagerduty/main.go:288
DEBU   Starting PagerDuty API health check... pagerduty/main.go:170
DEBU   Starting secure HTTPS server on :8081 utils/http.go:146
DEBU   Watcher connected pagerduty/main.go:252
DEBU   PagerDuty API health check finished ok pagerduty/main.go:176
DEBU   Setting up the webhook extensions pagerduty/main.go:178
```

By default, `teleport-pagerduty` will assume its config is in `/etc/teleport-pagerduty.toml`, but you can override it with `--config` option.

### Setup with SystemD
In production, we recommend starting teleport plugin daemon via an init system like systemd . Here's the recommended Teleport Plugin service unit file for systemd: 

```bash
{!examples/systemd/plugins/teleport-pagerduty.service!}
```

Save this as `teleport-pagerduty.service`. 

#### Example PagerDuty Request

<video  style="width:100%" controls>
  <source src="/img/enterprise/plugins/pagerduty/pagerduty-demo.mp4" type="video/mp4">
  <source src="/img/enterprise/plugins/pagerduty/pagerduty-demo.webm" type="video/webm">
Your browser does not support the video tag.
</video>