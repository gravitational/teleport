# Teleport Slack Plugin Setup Quickstart

If you're using Slack, you can be notified of [new teleport permission requests](https://gravitational.com/teleport/docs/cli-docs/#tctl-request-ls), approve or deny them on Slack with Teleport Slack Plugin. This guide covers it's setup.

For this quickstart, we assume you've already setup an [Enterprise Teleport Cluster](https://gravitational.com/teleport/docs/enterprise/quickstart-enterprise/)

!!! tip  
    The Approval Workflow only works with Pro and Enterprise versions of Teleport

## Prerequisites
- An Enterprise or Pro Teleport Cluster
- Admin Privileges. With access and control of [`tctl`](https://gravitational.com/teleport/docs/cli-docs/#tctl)
- Slack Admin Privileges to create an app and install it to your workspace.

#### Create User and Role for access. 
Log into Teleport Authentication Server, this is where you normally run `tctl`. Don't change the username and the role name, it should be `access-plugin` for the plugin to work correctly.

_Note: if you're using other plugins, you might want to create different users and roles for different plugins_.

```
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
  
### Create Slack App

We'll create a new Slack app and setup auth tokens and callback URLs, so that Slack knows how to notify the Teleport plugin when Approve / Deny buttons are clicked.

You'll need to: 

1. Create a new app, pick a name and select a workspace it belongs to. 
2. Select “app features”: we'll enable interactivity and setup the callback URL here. 
3. Add OAuth Scopes. This is required by Slack for the app to be installed — we'll only need a single scope to post messages to your Slack account. 
4. Obtain OAuth token and callback signing secret for the Teleport plugin config. 

#### Creating a New Slack app

Visit [https://api.slack.com/apps](https://api.slack.com/apps) to create a new Slack App. 

**App Name:** Teleport<br>
**Development Slack Workspace:** Pick the workspace you'd like the requests to show up in.

![Create Slack App](/img/enterprise/plugins/Create-a-Slack-App.png)

#### Setup Interactive Components

When someone tries approving / denying a request on Slack and clicks an Approve / Deny button, Slack will send a request to the Teleport Slack Plugin. In Slack App's settings, you must provide the URL for Slack to use for this. 

This URL must match the URL setting in Teleport Plugin settings file (we'll cover that later), and be publicly accessible.

For now, just think of the URL you'll use and set it in the Slack App's settings screen in Features > Interactive Components > Request URL.

![Interactive Components](/img/enterprise/plugins/slack/Interactive-Components.png)

#### Selecting OAuth Scopes
On the App screen, go to “OAuth and Permissions” under Features in the sidebar menu. Then scroll to Scopes, and add `chat.write` scope so that our plugin can post messages to your Slack channels.

#### Add to Workspace

![OAuth Tokens](/img/enterprise/plugins/slack/Slackbot-Permissions.png)

#### Obtain OAuth Token 

![OAuth Tokens](/img/enterprise/plugins/slack/OAuth.png)


#### Getting the secret signing token
In the sidebar of the app screen, click on Basic. Scroll to App Credentials section, and grab the app's Signing Secret. We'll use it in the config file later.

![Secret Signing Token](/img/enterprise/plugins/slack/SlackSigningSecret.png)

## Installing the Teleport Slack Plugin
To start using Teleport Plugins, you will need the teleport-slackbot executable.  See the [README](README.md) for building the teleport-slackbot executable in the Setup section.  Place the executable in the appropriate /usr/bin or /usr/local/bin on the server installation.

### Configuration File
Teleport Slack plugin has its own config file in TOML format. Before starting the plugin, you'll need to generate (or just copy the one below) and edit that config.

To generate a config file, you can do this: 

```bash
teleport-slackbot configure > /etc/teleport-slackbot.toml
```

Note that it saves the config file to `/etc/teleport-slackbot.toml`. You'll be able to point the plugin to any config file path you want, but it'll pick up `/etc/teleport-slackbot.toml` by default. 

#### Editing the config file
In the Teleport section, use the certificates you've generated with `tctl auth sign` before. The plugin installer creates a folder for those certificates in `/var/lib/teleport/plugins/slackbot/` — so just move the certificates there and make sure the config points to them. 

In Slack section, use the OAuth token, signing token, setup the desired channel name. The listen URL is the URL the plugin will listen for Slack callbacks. 

Then set the plugin callback (where Slack sends its requests) to an address you like, and provide the TLS certificates for that http server to use. 

By default, Teleport Slack plugin will run with TLS on. 

```TOML
# Example Teleport Slack Plugin config file
[teleport]
auth-server = "example.com:3025"  # Teleport Auth Server GRPC API address
client-key = "/var/lib/teleport/plugins/slackbot/auth.key" # Teleport GRPC client secret key
client-crt = "/var/lib/teleport/plugins/slackbot/auth.crt" # Teleport GRPC client certificate
root-cas = "/var/lib/teleport/plugins/slackbot/auth.cas"   # Teleport cluster CA certs

[slack]
token = "api-token"       # Slack Bot OAuth token
secret = "signing-secret-value"   # Slack API Signing Secret
channel = "channel-name"  # Slack Channel name to post requests to

[http]
listen = ":8081"          # Slack interaction callback listener port
# https-key-file = "/var/lib/teleport/plugins/slackbot/server.key"  # TLS private key
# https-cert-file = "/var/lib/teleport/plugins/slackbot/server.crt" # TLS certificate

[log]
output = "stderr" # Logger output. Could be "stdout", "stderr" or "/var/lib/teleport/slackbot.log"
severity = "INFO" # Logger severity. Could be "INFO", "ERROR", "DEBUG" or "WARN".
```

## Test Run 

Assuming that Teleport is running, and you've created the Slack app, the plugin config, and provided all the certificates — you can now run the plugin and test the workflow!

`teleport-slackbot start`

If everything works fine, the log output should look like this:

```bash
vm0:~/slackbot sudo ./teleport=slackbot start
INFO   Starting Teleport Access Slackbot 0.1.0: slackbot/main.go:224
INFO   Starting a request watcher... slackbot/main.go:330
INFO   Starting insecure HTTP server on 0.0.0.0:8081 utils/http.go:64
INFO   Watcher connected slackbot/main.go:298
```

### Testing the approval workflow

You can create a test permissions request with `tctl` and check if the plugin works as expected like this: 

#### Create a test permissions request

```bash
tctl request create USERNAME --roles=TARGET_ROLE
```

#### Check that you see a request message on Slack 

It should look like this: %image%

#### Approve or deny the request on Slack

The messages should automatically get updated to reflect the action you just clicked. 

You can also check the request status with `tctl`: 

```bash
tctl request ls
```

### TSH User Login and Request Admin Role. 

You can also test the full workflow from the user's perspective using `tsh`: 

```bash
➜ tsh login --request-roles=REQUESTED_ROLE
Seeking request approval... (id: 8f77d2d1-2bbf-4031-a300-58926237a807)
```

You should now see a new request in Teleport, and a message about the request on Slack. You can approve or deny it and `tsh`should login successfully or error out right after you click an action button on Slack.


### Setup with SystemD
In production, we recommend starting teleport plugin daemon via an init system like systemd . Here's the recommended Teleport Plugin service unit file for systemd: 

```service
{!examples/systemd/plugins/teleport-slackbot.service!}
```

Save this as `teleport-slackbot.service`. 
