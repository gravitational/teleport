# Teleport Slack Plugin Setup
This guide will talk through how to setup Teleport with Slack. Teleport to Slack integration allows you to treat Teleport access and permission requests via Slack message and inline interactive components.

!!! warning
    The Approval Workflow only works with Teleport Enterprise as it requires several roles.

#### Example Slack Request

<video style="width:100%" controls>
  <source src="/img/enterprise/plugins/slack/slack.mp4" type="video/mp4">
  <source src="/img/enterprise/plugins/slack/slack.webm" type="video/webm">
Your browser does not support the video tag.
</video>

## Setup
### Prerequisites
This guide assumes that you have:

* Teleport Enterprise 4.2.8 or newer
* Admin privileges with access to `tctl`
* Slack Admin Privileges to create an app and install it to your workspace.

#### Create User and Role for access.
Log into Teleport Authentication Server, this is where you normally run `tctl`. Create a
new user and role that only has API access to the `access_request` API. The below script
will create a yaml resource file for a new user and role.

```bash
# Copy and Paste the below on the Teleport Auth server.
$ cat > rscs.yaml <<EOF
kind: user
metadata:
  name: access-plugin-slack
spec:
  roles: ['access-plugin-slack']
version: v2
---
kind: role
metadata:
  name: access-plugin-slack
spec:
  allow:
    rules:
      - resources: ['access_request']
        verbs: ['list','read','update']
    # teleport currently refuses to issue certs for a user with 0 logins,
    # this restriction may be lifted in future versions.
    logins: ['access-plugin-slack']
version: v3
EOF

# Use tctl to create the user and role within Teleport.
$ tctl create -f rscs.yaml
```
!!! tip
    If you're using other plugins, you might want to create different users and roles for different plugins

#### Export access-plugin Certificate
Teleport Plugin use the `access-plugin-slack` role and user to perform the approval. We export the identity files, using [`tctl auth sign`](https://gravitational.com/teleport/docs/cli-docs/#tctl-auth-sign).

```bash
$ tctl auth sign --format=tls --user=access-plugin-slack --out=auth --ttl=8760h
# ...
```
The above sequence should result in three PEM encoded files being generated: auth.crt, auth.key, and auth.cas (certificate, private key, and CA certs respectively).  We'll reference these later when [configuring Teleport-Plugins](#configuring-teleport-slack).

!!! note "Certificate Lifetime"
     By default, [`tctl auth sign`](https://gravitational.com/teleport/docs/cli-docs/#tctl-auth-sign) produces certificates with a relatively short lifetime. For production deployments, the `--ttl` flag can be used to ensure a more practical certificate lifetime. `--ttl=8760h` exports a 1 year token

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

<a href="/img/enterprise/plugins/teleport_bot@2x.png" download>Download Teleport Bot Icon</a>

![Create Slack App](../../img/enterprise/plugins/slack/Create-a-Slack-App.png)

#### Setup Interactive Components

This URL must match the URL setting in Teleport Plugin settings file (we'll cover that later), and be publicly accessible.

For now, just think of the URL you'll use and set it in the Slack App's settings screen in Features > Interactive Components > Request URL.

![Interactive Components](../../img/enterprise/plugins/slack/setup-interactive-component.png)

#### Selecting OAuth Scopes
On the App screen, go to “OAuth and Permissions” under Features in the sidebar menu. Then scroll to Scopes, and add `incoming-webhook, users:read, users:read.email` scopes so that our plugin can post messages to your Slack channels.

![API Scopes](../../img/enterprise/plugins/slack/api-scopes.png)



#### Obtain OAuth Token

![OAuth Tokens](../../img/enterprise/plugins/slack/OAuth.png)

#### Getting the secret signing token
In the sidebar of the app screen, click on Basic. Scroll to App Credentials section, and grab the app's Signing Secret. We'll use it in the config file later.

![Secret Signing Token](../../img/enterprise/plugins/slack/SlackSigningSecret.png)

#### Add to Workspace

![OAuth Tokens](../../img/enterprise/plugins/slack/Slackbot-Permissions.png)
After adding to the workspace, you still need to invite the bot to the channel. Do this by using the @ command,
and inviting them to the channel.
![Invite bot to channel](../../img/enterprise/plugins/slack/invite-user-to-team.png)


## Installing the Teleport Slack Plugin

We recommend installing the Teleport Plugins alongside the Teleport Proxy. This is an ideal
location as plugins have a low memory footprint, and will require both public internet access
and Teleport Auth access.  We currently only provide linux-amd64 binaries, you can also
compile these plugins from [source](https://github.com/gravitational/teleport-plugins/tree/master/access/slack).

```bash
$ wget https://get.gravitational.com/teleport-access-slack-v{{ teleport.plugin.version }}-linux-amd64-bin.tar.gz
$ tar -xzf teleport-access-slack-v{{ teleport.plugin.version }}-linux-amd64-bin.tar.gz
$ cd teleport-access-slack/
$ ./install
$ which teleport-slack
/usr/local/bin/teleport-slack
```

Run `./install` in from 'teleport-slack' or place the executable in the appropriate `/usr/bin` or `/usr/local/bin` on the server installation.

### Configuring Teleport Slack

Teleport Slack uses a config file in TOML format. Generate a boilerplate config by
running the following command:

```bash
$ teleport-slack configure > teleport-slacbot.toml
$ sudo mv teleport-slack.toml /etc
```

Then, edit the config as needed.

```yaml
{!examples/resources/plugins/teleport-slack.toml!}
```

#### Editing the config file
In the Teleport section, use the certificates you've generated with `tctl auth sign` before. The plugin installer creates a folder for those certificates in `/var/lib/teleport/plugins/slack/` — so just move the certificates there and make sure the config points to them.

In Slack section, use the OAuth token, signing token, setup the desired channel name. The listen URL is the URL the plugin will listen for Slack callbacks.

Then set the plugin callback (where Slack sends its requests) to an address you like, and provide the TLS certificates for that http server to use.

By default, Teleport Slack plugin will run with TLS on.

```conf
{!examples/resources/plugins/teleport-slack.toml!}
```
## Test Run

Assuming that Teleport is running, and you've created the Slack app, the plugin config,
and provided all the certificates — you can now run the plugin and test the workflow!

```bash
$ teleport-slack start
```
If everything works fine, the log output should look like this:

```bash
$ teleport-slack start
INFO   Starting Teleport Access Slack {{ teleport.plugin.version }}.1-0-slack/main.go:145
INFO   Starting a request watcher... slack/main.go:330
INFO   Starting insecure HTTP server on 0.0.0.0:8081 utils/http.go:64
INFO   Watcher connected slack/main.go:298
```
### Testing the approval workflow

You can create a test permissions request with `tctl` and check if the plugin works as expected like this:

#### Create a test permissions request behalf of a user.

```bash
# Replace USERNAME with a local user, and TARGET_ROLE with a Teleport Role
$ tctl request create USERNAME --roles=TARGET_ROLE
```
A user can also try using `--request-roles` flag.
```bash
# Example with a user trying to request a role DBA.
$ tsh login --request-roles=dba
```

#### Check that you see a request message on Slack

It should look like this: TODO

#### Approve or deny the request on Slack

The messages should automatically get updated to reflect the action you just clicked. You can also check the request status with `tctl`:

```bash
$ tctl request ls
```

### TSH User Login and Request Admin Role.

You can also test the full workflow from the user's perspective using `tsh`:

```bash
# tsh login --request-roles=REQUESTED_ROLE
Seeking request approval... (id: 8f77d2d1-2bbf-4031-a300-58926237a807)
```
You should now see a new request in Teleport, and a message about the request on Slack. You can
approve or deny it and `tsh` should login successfully or error out right after you click an action button on Slack.

### Setup with SystemD
In production, we recommend starting teleport plugin daemon via an init system like systemd .
Here's the recommended Teleport Plugin service unit file for systemd:

```bash
{!examples/systemd/plugins/teleport-slack.service!}
```
Save this as `teleport-slack.service`.

## Audit Log
The plugin will let anyone with access to the Slack Channel so it's
important to review Teleport' audit log.

## Feedback
If you have any issues with this plugin please create an [issue here](https://github.com/gravitational/teleport-plugins/issues/new).