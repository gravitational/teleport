# Teleport Jira Server Plugin Setup
This guide will talk through how to setup Teleport with Jira Server. Teleport to Jira Server integration allows you to treat Teleport access and permission requests as Jira Tasks.

!!! warning
    The Approval Workflow only works with Teleport Enterprise as it requires several roles.

!!! note
    Teleport's tsh request workflow is synchronous and needs to be approved within 1 hour of the request.

<video style="width:100%" controls>
  <source src="/img/enterprise/plugins/jira/jira-server.mp4" type="video/mp4">
  <source src="/img/enterprise/plugins/jira/jira-server.webm" type="video/webm">
Your browser does not support the video tag.
</video>

## Setup
### Prerequisites
* An Enterprise or Pro Teleport Cluster
* Admin Privileges with access and control of [`tctl`](https://gravitational.com/teleport/docs/cli-docs/#tctl)
* Jira Server installation with owner privileges, specifically to setup webhooks, issue types, and workflows. This plugin has been tested with Jira Software 8.8.0

### Create an access-plugin role and user within Teleport
First off, using an existing Teleport Cluster, we are going to create a new Teleport User and Role to access Teleport.

#### Create User and Role for access.
Log into Teleport Authentication Server, this is where you normally run `tctl`. Create a
new user and role that only has API access to the `access_request` API. The below script
will create a yaml resource file for a new user and role.

```bash
$ cat > rscs.yaml <<EOF
kind: user
metadata:
  name: access-plugin-jira
spec:
  roles: ['access-plugin']
version: v2
---
kind: role
metadata:
  name: access-plugin-jira
spec:
  allow:
    rules:
      - resources: ['access_request']
        verbs: ['list','read','update']
    # teleport currently refuses to issue certs for a user with 0 logins,
    # this restriction may be lifted in future versions.
    logins: ['access-plugin-jira']
version: v3
EOF

# ...
$ tctl create -f rscs.yaml
```

#### Export access-plugin Certificate
Teleport Plugin uses the `access-plugin-jira` role and user to perform the approval. We export the identity files, using [`tctl auth sign`](https://gravitational.com/teleport/docs/cli-docs/#tctl-auth-sign).

```bash
$ tctl auth sign --format=tls --user=access-plugin-jira --out=auth --ttl=8760h
# ...
```

The above sequence should result in three PEM encoded files being generated: auth.crt, auth.key, and auth.cas (certificate, private key, and CA certs respectively).  We'll reference these later when [configuring Teleport-Plugins](#configuration-file).

!!! note "Certificate Lifetime"
     By default, [`tctl auth sign`](https://gravitational.com/teleport/docs/cli-docs/#tctl-auth-sign) produces certificates with a relatively short lifetime. For production deployments, the `--ttl` flag can be used to ensure a more practical certificate lifetime. `--ttl=8760h` exports a 1 year token

### Setting up your Jira Server instance

#### Creating a Project
Teleport Jira Plugin relies on your Jira project having a board with at least three statuses (columns): Pending, Approved, and Denied. It's therefore the easiest scenario to create a new Jira project for Teleport to use.

The specific type of project you choose when you create it doesn't matter, as long as you can setup a Kanban Board for it, but we recommend that you go with Kanban Software Development — this will reduce the amount of setup work you'll have to do and provide the board out of the box.

You'll need the project key for the Teleport plugin settings later on. It's usually a 3 character code for the project.

#### Setting up Request ID field on Jira
Teleport stores the request metadata in a special Jira custom field that must be named teleportAccessRequestId. To create that field, go to Administration -> Issues -> Custom Fields -> Add Custom Field.

Name the field `teleportAccessRequestId`, and choose Text Field (single line) as the field type. Assign the field to your project, or make it global. Teleport Access Request ID is an internal field and it's not supposed to be edited by users, so you can leave the Screens section blank. That means that the field won't show up in Jira UI.

Go to Project Settings -> Fields and make sure that the `teleportAccessRequestId` field shows up on the list of fields available in this project.

#### Setting up the status board
The default Jira Software workflow has a different board setup from what Teleport needs, so we'll setup another workflow and assign that workflow to the project board.

Go to Administration -> Workflows. You can choose to add a new workflow (recommended), or edit the existing workflow, it'll be called Software Simplified Workflow for Project NAME by default. It's only used in your single project, so it's safe to edit it.

Edit the workflow to have these three states:

1. Pending
2. Approved
3. Denied
The rules of the workflow must meet these requirements:

- New created issues should be in Pending state.
- It should be possible to move from Pending to Approved
- It should be possible to move from Pending to Declined.
- You can choose to make the workflow strict and restrict moving requests from Approved state to Declined state and vice versa, or leave that flexible. Teleport will only change the request status once, i.e. the first time the request is approved or denied on your Jira board.

With Workflow editor you can setup who can approve or deny the request based on their Jira user permissions. We won't cover that in this guide as it mostly relates to Jira settings. By default Teleport will allow anyone who can use the workflow to approve or deny the request.

Go to your Project Settings -> Workflows, and make sure that your workflow that you just created or edited is applied to the project you'll use for Teleport integration.

### Setting up the webhook

Teleport Jira Plugin will listen for a webhook that Jira Server sends when a request is approved or denied. Go to Settings -> System -> Webhooks to setup the webhook. The webhook needs to be sent when issues are updated or deleted.

#### Configuring the Teleport Jira Plugin for Jira Server

## Installing
We recommend installing the Teleport Plugins alongside the Teleport Proxy. This is an ideal
location as plugins have a low memory footprint, and will require both public internet access
and Teleport Auth access.  We currently only provide linux-amd64 binaries, you can also
compile these plugins from [source](https://github.com/gravitational/teleport-plugins/tree/master/access/jira).

```bash
$ wget https://get.gravitational.com/teleport-access-jira-v{{ teleport.plugin.version }}-linux-amd64-bin.tar.gz
$ tar -xzf teleport-access-jira-v{{ teleport.plugin.version }}-linux-amd64-bin.tar.gz
$ cd teleport-access-jira/
$ sudo ./install
# Teleport Jira Plugin binaries have been copied to /usr/local/bin
# You can run teleport-jira configure > /etc/teleport-jira.toml to bootstrap your config file.
$ which teleport-jira
/usr/local/bin/teleport-jira
```

Run `sudo ./install` in from 'teleport-jira' or place the executable in the appropriate `/usr/bin` or `/usr/local/bin` on the server installation.

### Configuration file
Teleport Jira Plugin uses a config file in TOML format. Generate a boilerplate config by
running the following command:

```bash
$ teleport-jira configure > teleport-jira.toml
$ sudo mv teleport-jira.toml /etc
```

By default, Jira Teleport Plugin will use a config in `/etc/teleport-jira.toml`, and you can override it with `-c config/file/path.toml` flag.

```toml
{!examples/resources/plugins/teleport-jira.toml!}
```

The `[teleport]` section describes where is the teleport service running, and what keys should the plugin use to authenticate itself. Use the keys that you've generated [above in exporting your Certificate section](#export-access-plugin-certificate).

The `[jira]` section requires a few things:
1. Your Jira Cloud or Jira Server URL. For Jira Cloud, it looks something like yourcompany.atlassian.net.
2. Your username on Jira, i.e. benarent **Note: Not your email address.**
3. Your Jira API token. **For Jira Server, this is a password.  it's a good idea to create a separate user record with permissions limited to accessing this particular project board, and use this with the bot.**
4. And the Jira Project key, available in Project settings.

`[http]` setting block describes how the Plugin's HTTP server works. The HTTP server is responsible for listening for updates from Jira, and processing updates, like when someone drags a task from Inbox to Approved column.

You must provide an address the server should listen on, and a certificate to use, unless you plan on running with `--insecure-no-tls`, which we don't recommend in production.


## Testing
You should be able to run the Teleport plugin now!

```bash
teleport-jira start
INFO   Starting Teleport Access JIRAbot 0.1.0-alpha.3:teleport-jira-v0.1.0-alpha.3-0-gea1ef8e jira/app.go:74
# DEBU   Checking Teleport server version jira/app.go:150
# DEBU   Starting JIRA API health check... jira/app.go:111
# DEBU   Checking out JIRA project... jira/bot.go:145
# DEBU   Found project "TEL1": "Tel-kb" jira/bot.go:150
# DEBU   Checking out JIRA project permissions... jira/bot.go:152
# DEBU   JIRA API health check finished ok jira/app.go:117
# DEBU   Starting secure HTTPS server on 66.66.66.66:8081 utils/http.go:235
# DEBU   Watcher connected access/service_job.go:62
```

The log output should look familiar to what Teleport service logs. You should see that it connected to Teleport, and is listening for new Teleport requests and Jira webhooks.

Go ahead and test it:

```bash
tsh login --request-roles=admin
```

That should create a new permission request on Teleport (you can test if it did with `tctl request ls` ), and you should see a new task on your Jira project board.

### Setup with SystemD
In production, we recommend starting teleport plugin daemon via an init system like systemd .
Here's the recommended Teleport Plugin service unit file for systemd:

```bash
{!examples/systemd/plugins/teleport-jira.service!}
```
Save this as `teleport-jira.service`.

## Audit Log
The plugin will let anyone with access to the Jira board approve or deny requests, so it's important to review Teleport's audit log.

## Feedback
If you have any issues with this plugin please create an [issue here](https://github.com/gravitational/teleport-plugins/issues/new).