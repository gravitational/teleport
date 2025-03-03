# Getting Started with Teleport Jira Plugin on Jira Cloud

If you're using Jira Cloud or Jira Server to manage your projects, you can also use it to monitor, approve, deny, or discuss Teleport permission requests. This quickstart will walk you through the setup.

For the purpose of this quickstart, we assume you've already setup an [Teleport Cluster](https://goteleport.com/docs/getting-started/).

## Prerequisites

1. Teleport Cluster with admin permissions. Make sure you're able to add roles and users to the cluster.
1. Jira Server or Jira Cloud installation with an owner privileges, specifically to setup webhooks, issue types, and workflows.

### Set up plugin role and user in Teleport.

It's covered in [README](./README.md).

### Setting up your Jira Project

#### Creating the permission management project

All new permission requests are going to show up in a project you choose. We recommend that you create a separate project for permissions management, and a new board in said project.

You'll need the project Jira key to configure the plugin.

#### Setting up the status board

Create a new board for tasks in the permission management project. The board has to have at least these three columns:
1. Pending
2. Approved
3. Denied

Teleport Jira Plugin will create a new issue for each new permission request in the first available column on the board. When you drag the request task to Approved column on Jira, the request will be approved. Ditto for Denied column on Jira.

#### Setting up Request ID field on Jira

Teleport Jira Plugin requires a custom issue field to be created.
Go to your Jira Project settings -> Issue Types -> Select type `Task` -> add a new Short Text field named `TeleportAccessRequestId`.
Teleport uses this field to reference it's internal request ID. If anyone changes this field on Jira, or tries to forge the permission request, Teleport will validate it and ignore it.

#### Getting your Jira API token

If you're using Jira Cloud, navigate to [Account Settings -> Security -> API Tokens](https://id.atlassian.com/manage/api-tokens) and create a new app specific API token in your Jira installation.
You'll need this token later to configure the plugin.

For Jira Server, the URL of the API tokens page will be different depending on your installation.

#### Settings up Jira Webhooks

Go to Settings -> General -> System -> Webhooks and create a new Webhook for Jira to tell the Teleport Plugin about updates.

For the webhook URL, use the URL that you'll run the plugin on. It needs to be a publicly accessible URL that we'll later setup.
Jira requires the webhook listener to run over HTTPS.

The Teleport Jira plugin webhook needs to be notified only about new issues being created, issues being updated, or deleted. You can leave all the other boxes empty.

_Note: by default, Jira Webhook will send updates about any issues in any projects in your Jira installation to the webhook.
We suggest that you use JQL filters to limit which issues are being sent to the plugin._

_Note: by default, the Plugin's web server will run with TLS, but you can disable it with `--insecure-no-tls` to test things out in a dev environment._

In the webhook settings page, make sure that the webhook will only send Issue Updated updates. It's not critical if anything else gets sent, the plugin will just ignore everything else.
