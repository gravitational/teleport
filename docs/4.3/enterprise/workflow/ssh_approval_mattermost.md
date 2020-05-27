# Teleport Mattermost Plugin Setup

This guide will talk through how to setup Teleport with Mattermost.  Teleport ↔ Mattermost integration  allows teams to approve or deny Teleport access requests using [Mattermost](https://mattermost.com/) an open source messaging platform. 

!!! warning
    The Approval Workflow only works with Teleport Enterprise as it's requires several roles.

## Setup
### Prerequisites

This guide assumes that you have: 

* Teleport Enterprise 4.2.8 or newer
* Admin privileges with access to `tctl`
* Mattermost account with admin privileges. This plugin has been tested with Mattermost 5.x 

#### Setting up Mattermost to work with the bot

![Enable Mattermost bots](/img/enterprise/plugins/mattermost/mattermost_admin_console_integrations_bot_accounts.png)

In Mattermost, go to System Console → Integrations → Enable Bot Account Creation → Set to True.
This will allow us to create a new bot account that the Teleport bot will use.

Go back to your team, then Integrations → Bot Accounts → Add Bot Account.

The new bot account will need Post All permission. 

<a href="/img/enterprise/plugins/teleport_bot@2x.png" download>Download Teleport Bot Icon</a>


![Enable Mattermost Bots](/img/enterprise/plugins/mattermost/mattermost_bot.png)

##### Create an OAuth 2.0 Application

In Mattermost, go to System Console → Integrations → OAuth 2.0 Applications. 
- Set Callback URLs to the location of your Teleport Proxy

![Create OAuth Application](/img/enterprise/plugins/mattermost/mattermost_OAuth_token.png)

The confirmation screen after you've created the bot will give you the access token.
We'll use this in the config later.

#### Create User and Role for access. 
Log into Teleport Authentication Server, this is where you normally run `tctl`. Don't change the username and the role name, it should be `access-plugin` for the plugin to work correctly.

_Note: If you're using other plugins, you might want to create different users and roles for different plugins_.

```yaml
# This command will create two Teleport Yaml resources, a new Teleport user and a 
# Role for that users that can only approve / list requests. 
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

# Run this to create the user and role in Teleport. 
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

## Downloading and installing the plugin

The recommended way to run Teleport Mattermost plugin is by downloading the release version and installing it: 

```bash
$ wget https://get.gravitational.com/teleport-mattermost-v{{ teleport.plugin.version }}-linux-amd64-bin.tar.gz
$ tar -xzf teleport-mattermost-v{{ teleport.plugin.version }}-linux-amd64-bin.tar.gz
$ cd teleport-mattermost
$ ./install
$ which teleport-mattermost
/usr/local/bin/teleport-mattermost
```

### Configuring Mattermost bot

Mattermost Bot uses a config file in TOML format. Generate a boilerplate config by 
running the following command: 

```bash
teleport-mattermost configure > /etc/teleport-mattermost.toml
```

Then, edit the config as needed.

```yaml
{!examples/resources/plugins/teleport-mattermost.toml!}
```

### Testing the Plugin

With the config above, you should be able to run the bot invoking 
`teleport-mattermost start -d`. The will provide some debug information to make sure
the bot can connect to Mattermost. 

```bash
$ teleport-mattermost start -d
DEBU   DEBUG logging enabled logrus/exported.go:117
INFO   Starting Teleport Access Mattermost Bot {{ teleport.plugin.version }}-dev.1: mattermost/main.go:140
DEBU   Checking Teleport server version mattermost/main.go:234
DEBU   Starting a request watcher... mattermost/main.go:296
DEBU   Starting Mattermost API health check... mattermost/main.go:186
DEBU   Starting secure HTTPS server on :8081 utils/http.go:146
DEBU   Watcher connected mattermost/main.go:260
DEBU   Mattermost API health check finished ok mattermost/main.go:19
```

### Setup with SystemD
In production, we recommend starting teleport plugin daemon via an init system like systemd . Here's the recommended Teleport Plugin service unit file for systemd: 

```bash
{!examples/systemd/plugins/teleport-mattermost.service!}
```

Save this as `teleport-mattermost.service`. 