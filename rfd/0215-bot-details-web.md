---
authors: Nick Marais (nicholas.marais@goteleport.com)
state: draft
---
# RFD0215 - Bot Details (web)

## Required Approvers

* Engineering: @strideynet
* Product: @thedevelopnik && (samrat || kenny) <-- TODO: find GitHub names

## What

Add a page to the web app which provides details for a single bot. This page is linked to from the existing bots list. Features of the page will be delivered incrementally.

In the first iteration, the page will show basic details (name, created at, max ttl, etc), roles, traits, linked join methods and *active* bot instances. Deleting and locking the bot is allowed, as well as editing role assignment, allowed logins and max session duration (ttl) - this matches the operations available using `tctl`.

The feature set of subsequent iterations remains flexible to allow customer and community feedback to shape the future of the product. This document will be updated to reflect future iterations as they are planned and implemented. A [[#Wishlist features|wish list of features]] is included in this document.
## Why

Management operations on bots are only possible via `tclt` - this change seeks to make these operations more accessible and more friendly for non-technical users (i.e. users less comfortable on the command line). The new page is targeted mainly at members of the Infrastructure Security team whose role it is to configure and maintain a Teleport deployment, as well as enrol protected resources.

## Details

### Day 1 vs. day 2
- No expected differences between day 1 and day 2 experience?
- Is there a day 1 mode where the bot has zero use, where extra enrol info can be provided, and join tokens configured?

### User stories

As a member of the **Infrastructure team**,
I would like to **view information about a bot** (such as name and create/updated at & by),
So that I can track changes to the bot’s configuration for auditing or troubleshooting purposes.

As a member of the **Infrastructure team**,
I would like to link out to documentation about **what a bot is and how it works**,
So that I can get the most out of the capability and feel confident deploying a solution leveraging bots.

As a member of the **Infrastructure team**,
I would like to **edit the roles assigned** to a bot,
So that I can easily extend or reduce the scope of a bot without the need to migrate existing agents (e.g. by needing to recreate the bot).

As a member of the **Infrastructure team**,
I would like to **delete a bot** when it is no longer required,
So that I can reduce unnecessary access paths to resources, and keep the cluster configuration tidy and free of historic clutter.

As a member of the **Infrastructure team**,
I would like to **lock a bot** and all instances,
So that current and future access to protected resources is immediately prevented.

As a member of the **Infrastructure team**,
I would like to **view assigned roles** for a bot,
So that I can easily determine the scope of access at a glance.

As a member of the **Infrastructure team**,
I would like to **view configured join methods** for a bot,
So that I can easily determine the enrolment mechanisms available at a glance.

As a member of the **Infrastructure team**,
I would like to see a list of **currently active** instances for a bot with **last active times** and high-level characteristics (such as **bot name**, **hostname** and **join method**),
So that I can assess the usage of the bot by scanning over the results to build an overall picture of recency and a distribution of characteristics.

### UX

![[0215-iteration-1.png]]

#### Bot info and config

Shows basic details and configuration. All items are readonly. Date/time items have a hover state which shows a tooltip with the full date and time.

![[0215-feature-info.png]]
**Data source**
``` yaml
# Bot resource (calculated)
kind: bot
metadata:
  name: robot
spec:
  max_session_ttl:
    seconds: 43200
status:
  role_name: bot-robot # Links to role
  user_name: bot-robot # Links to user

# User resource
kind: user
metadata:
  labels:
    teleport.internal/bot: robot # Links to bot
  name: bot-robot
spec:
  created_by:
    time: "2025-06-02T11:25:13.238653583Z"
    user:
      name: nicholas.marais@goteleport.com
  roles:
  - bot-robot # Links to role
  status:
    is_locked: false
    lock_expires: "0001-01-01T00:00:00Z"
    locked_time: "0001-01-01T00:00:00Z"

# Role resource
kind: role
metadata:
  name: bot-robot
```
#### Join tokens
Lists Join Tokens with a role of "Bot" and `bot_name` matching the bot being viewed. An overflow menu allow navigating to Zero Trust Access > Join Tokens. Clicking an item navigates to to the view/edit page for that token.

![[0215-feature-join-tokens.png]]
**Data source**
``` yaml
# Token resource
kind: token
metadata:
  name: robot-github
spec:
  bot_name: robot
  join_method: github
  roles:
  - Bot
```
#### Roles and traits
Provides a full list of traits (internally recognised and custom) and allows the list to be filtered by role.

![[0215-feature-roles-traits.png]]

**Data source**
``` yaml
# User resource (bot)
kind: user
metadata:
  labels:
    teleport.internal/bot: robot
  name: bot-robot
spec:
  roles:
  - bot-robot
  traits:
    logins:
    - nick.marais

# Role resource (bot)
kind: role
metadata:
  labels:
    teleport.internal/bot: robot
  name: bot-robot
spec:
  allow:
    impersonate:
      roles:
      - file-browser-access
      - mac.lan-ssh-access

# Role resource
kind: role
metadata:
  name: file-browser-access
spec:
  allow:
    app_labels:
      file-browser: full

# Role resource
kind: role
metadata:
  name: access
spec:
  allow:
    app_labels:
      '*': '*'
    aws_role_arns:
    - '{{internal.aws_role_arns}}'
    azure_identities:
    - '{{internal.azure_identities}}'
    db_labels:
      '*': '*'
    db_names:
    - '{{internal.db_names}}'
    db_roles:
    - '{{internal.db_roles}}'
    db_service_labels:
      '*': '*'
    db_users:
    - '{{internal.db_users}}'
    gcp_service_accounts:
    - '{{internal.gcp_service_accounts}}'
    github_permissions:
    - orgs:
      - '{{internal.github_orgs}}'
    kubernetes_groups:
    - '{{internal.kubernetes_groups}}'
    kubernetes_labels:
      '*': '*'
    kubernetes_resources:
    - kind: '*'
      name: '*'
      namespace: '*'
      verbs:
      - '*'
    kubernetes_users:
    - '{{internal.kubernetes_users}}'
    logins:
    - '{{internal.logins}}'
    node_labels:
      '*': '*'
    windows_desktop_labels:
      '*': '*'
    windows_desktop_logins:
    - '{{internal.windows_logins}}'
```
#### Active instances
Lists the most recent (max 10) instances for the bot, ordered most recent first. A "see more" action navigates to the bot instances page with a pre-populated search filter on the bot's name - this is an imperfect filter as it's a contains-text filter across all fields

![[0215-feature-active-instances.png]]
#### Delete bot
Deletes the bot after confirmation. Shows a loading indicator during the call to the api. On success, navigates to the bots list (`/web/bots`). On error, shows a message within the confirmation dialog.

![[0215-feature-delete-bot.png]]
#### Lock bot
Locks the bot after confirmation. Shows a loading indicator during the call to the api. On success, removes the dialog but remains on the bot detail page. On error, shows a message within the confirmation dialog.

![[0215-feature-lock-bot.png]]
#### Edit roles
Shows a dialog where the user can add and/or remove assigned roles.
#### Edit logins
Shows a dialog where the user can add and/or remove allowed logins (server access/ssh).
#### Edit max session duration (`max_session_ttl`)
Shows a dialog where the user can edit the configured max session duration. Input is in the form `43200s`, `30m` or `3h`.
### Implementation

#### Data fetching and caching

In order to keep the implementation modular, each panel fetches its own data, shows loading status and manages error states. This increases flexibility for future iterations and reduces the likelihood of introducing regression issues. Data caching can be tailored to each area of data.

#### Web APIs

| **New**                                                     |                                                                                                                              |
| ----------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Fetch join tokens linked to a bot by name                   | `GET /v1/webapi/sites/:site/machine-id/bot/:name/token`                                                                      |
| Fetch roles and traits for a bot by name                    | `GET /v1/webapi/sites/:site/machine-id/bot/:name/trait`                                                                      |
| Update logins and lock state - editing roles already exists | `PUT /v1/webapi/sites/:site/machine-id/bot/:name`                                                                            |
|                                                             |                                                                                                                              |
| **Existing**                                                |                                                                                                                              |
| Fetch a bot by name                                         | `GET /v1/webapi/sites/:site/machine-id/bot/:name`                                                                            |
| Delete a bot                                                | `DELETE /v1/webapi/sites/:site/machine-id/bot/:name`                                                                         |
| Fetch active instances for a bot by name                    | `GET /v1/webapi/sites/:site/machine-id/bot-instance?search=:bot-name&page_size=20&page=/bot-instance/:bot_name/:instance_id` |

#### UI

Implementation of the UI will make heavy use of existing shared components. No new design components will be contributed.

#### Feature Flag

A feature flag should be used to allow partial features to be build and merged without worrying about incomplete features being released to customers.
#### Tasks
1. Enable deeplinking to Join Token view/edit page
2. Endpoint `GET /v1/webapi/sites/:site/machine-id/bot/:name/token`
3. Endpoint `GET /v1/webapi/sites/:site/machine-id/bot/:name/trait`
4. Endpoint `PUT /v1/webapi/sites/:site/machine-id/bot/:name`
5. UI panel component
6. Bot info panel
7. Join tokens panel
8. Roles and traits panel
9. Active instances panel
10. Delete bot operation
11. Lock bot operation
12. Edit roles operation
13. Edit logins operation
14. Edit max session duration operation
### Wishlist features

#### Recent instances (historic)

![[0215-feature-recent-intances.png]]
#### Audit log

![[0215-feature-audit-log.png]]
#### Access Graph

![[0215-feature-access-graph.png]]

#### Session recordings

![[0215-feature-session-recordings.png]]

#### Activity visualisation

![[0215-feature-activity-timeseries.png]]