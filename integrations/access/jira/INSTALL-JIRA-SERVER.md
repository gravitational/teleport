# Getting Started with Teleport Jira Plugin on Jira Server

This guide ia an additional guide to the [README.md](./README.md) and [INSTALL-JIRA-CLOUD.md](./INSTALL-JIRA-CLOUD.md).
It covers the differences in the setup process for the Teleport Jira Plugin for Jira Server environments.
Because Jira Server works with a different project and kanban board setup, several more steps are required to set it up to work with the plugin.

## Prerequisites

1. Teleport Cluster with admin permissions. Make sure you're able to add roles and users to the cluster.
1. A Jira Server instance. Your user must be an owner of the instance to set it up.

_Note:Setting up a playground:_: Before you setup the plugin in your production environment, you can setup a sandboxed jira server environment to and go through the setup process to make sure it works correctly.
To do that, you could [run Jira Server in a Docker container](https://hub.docker.com/r/atlassian/jira-software) and go through the setup steps.

## Setting up the Jira Server instance

For the rest of the guide, we assume you have access to a Jira Server instance.

### Creating a project

Teleport Jira Plugin relies on your Jira project having a board with at least three statuses (columns): Pending, Approved, and Denied. It's therefore the easiest scenario to create a new Jira project for Teleport to use.

Specific type of the project you choose when you create it doesn't matter, as long as you can setup a Kanban Board for it, but we recommend that you go with Kanban Software Development — this will reduce the amount of setup work you'll have to do and provide the board out of the box.

You'll need the project key for the teleport plugin settings later on. It's usually a 3 character code for the project.

### Adding a custom field

Teleport stores the request metadata in a special Jira custom field that must be named `teleportAccessRequestId`.
To create that field, go to Administration -> Issues -> Custom Fields -> Add Custom Field.

Name the field `teleportAccessRequestId`, and choose Text Field (single line) as the field type.
Assign the field to your project, or make it global.
Teleport Access Request ID is an internal field and it's not supposed to be edited by users, so you can leave the Screens section blank. That means that the field won't show up in Jira UI.

Go to Project Settings -> Fields and make sure that the teleportAccessRequestId field shows up on the list of fields available in this project.

### Setting up the workflow

The default Jira Software workflow has a different board setup from what Teleport needs, so we'll setup another workflow and assign that workflow to the project board.

Go to Administration -> Workflows. You can choose to add a new workflow (recommended), or edit the existing workflow, it'll be called `Software Simplified Workflow for Project NAME` by default. It's only used in your single project, so it's safe to edit it.

Edit the workflow to have these three states:
1. Pending
2. Approved
3. Denied

The rules of the workflow must meet these requirements:
- New created issues should be in Pending state.
- It should be possible to move from Pending to Approved
- It should be possible to move from Pending to Declined.

You can choose to make the workflow strict and restrict moving requests from Approved state to Declined state and vice versa, or leave that flexible. Teleport will only change the request status once, i.e. the first time the request is approved or denied on your Jira board.

With W editor you can setup who can approve or deny the request based on their Jira user permissions. We won't cover that in this guide as it mostly relates to Jira settings and Teleport will by default allow anyone who can use the workflow to approve or deny the request.

Go to your Project Settings -> Workflows, and make sure that your workflow that you just created or edited is applied to the project you'll use for Teleport integration.

### Setting up the webhook

Teleport Jira Plugin will listen for a webhook that Jira Server sends when a r is approved or denied. Go to Settings -> System -> Webhooks to setup the webhook. The webhook needs to be sent when issues are updated or deleted.

## Configuring the Teleport Jira Plugin for Jira Server

Once you've successfully setup your Jira Server, you'll need to tweak your `teleport-jira.toml` file.

- Jira URL: use your Jira Server root url, not your project dashboard url.
- Project key: your project key.
- username: your Jira Server username. **note: not your email address**.
- password: your Jira account password. **note: it's a good idea to create a separate user record with permissions limited to accessing this particular project board, and use this with the bot.**


When you start `teleport-jira`, it'll perform an API health check and verify that your Jira Server instance is compatible with the plugin, and that your username and password are authorized.
