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

The feature set of subsequent iterations remains flexible to allow customer and community feedback to shape the direction of the product. This document will be updated to reflect future iterations as they are planned and implemented. A [[#Wishlist features|wish list of features]] is included.
## Why

Management operations and diagnostic information on bots is only possible via `tclt` - this change seeks to make these more accessible and more friendly for non-technical users (i.e. users less comfortable on the command line). The new page is targeted mainly at members of the Infrastructure Security team whose role it is to configure and maintain a Teleport deployment, as well as enrol protected resources.

## Details

### Day 1 vs. day 2
In it's first increment the Bot Details page has no expected differences between day 1 and day 2 experiences, and it's likely to be used by users who are already acquainted with Teleport.

### User stories

As a member of the **Infrastructure Security team**,
I would like to **view information about a bot** (such as name and create/updated at & by),
So that I can track changes to the bot’s configuration for auditing or troubleshooting purposes.

As a member of the **Infrastructure Security team**,
I would like to link out to documentation about **what a bot is and how it works**,
So that I can get the most out of the capability and feel confident deploying a solution leveraging bots.

As a member of the **Infrastructure Security team**,
I would like to **edit the assigned roles, allowed traits and max session dutration** for a bot,
So that I can easily extend or reduce the scope of a bot without the need to migrate existing agents (e.g. by needing to recreate the bot) or using `tctl`.

As a member of the **Infrastructure Security team**,
I would like to **delete a bot** when it is no longer required,
So that I can reduce unnecessary access paths to resources, and keep the cluster configuration tidy and free of historic clutter.

As a member of the **Infrastructure Security team**,
I would like to **lock a bot** and all instances,
So that current and future access to protected resources is immediately prevented.

As a member of the **Infrastructure Security team**,
I would like to **view assigned roles and allowed traits** for a bot,
So that I can easily determine the scope of access at a glance.

As a member of the **Infrastructure Security team**,
I would like to **view configured join methods** for a bot,
So that I can easily determine the enrolment mechanisms available at a glance.

As a member of the **Infrastructure Security team**,
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
Provides full lists of roles and traits (well-known and custom). Edit operations are provided for each for convenience, which open the page-wise edit modal with all editable fields available.

![[0215-feature-roles-traits.png]]
**Data source**
``` yaml
# Bot resource
kind: bot
metadata:
  name: robot
spec:
  roles:
  - file-browser-access
  - mac.lan-ssh-access
  traits:
  - name: logins
    values:
    - nick.marais
```
#### Active instances
Lists the most recent (max 10) instances for the bot, ordered most recent first. A "see more" action navigates to the bot instances page with a pre-populated search filter on the bot's name - this is an imperfect filter as it's a contains-text filter across all fields

![[0215-feature-active-instances.png]]

#### Edit roles, traits and max session duration (`max_session_ttl`)
Shows a dialog where the user can add and/or remove assigned roles, add and/or remove traits (well-known or custom), and edit the configured max session duration in the form `43200s`, `30m` or `3h`. Allow all changes to be made in a single atomic transaction.

TODO: wireframe for edit form

#### Delete bot
Deletes the bot after confirmation. Shows a loading indicator during the call to the api. On success, navigates to the bots list (`/web/bots`). On error, shows a message within the confirmation dialog.

![[0215-feature-delete-bot.png]]
#### Lock bot
Locks the bot after confirmation. Shows a loading indicator during the call to the api. On success, removes the dialog but remains on the bot detail page. On error, shows a message within the confirmation dialog.

![[0215-feature-lock-bot.png]]
### Implementation

#### Data fetching and caching

In order to keep the implementation modular, each logical section of the page fetches its own data, shows loading status and manages error states. This increases flexibility for future iterations while reducing the likelihood of introducing regression issues. Data caching behaviour can be tailored to each area of data independently.

#### Web APIs

| **New**                                         |                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| ----------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Fetch join tokens linked to a bot by name       | `GET /v1/webapi/sites/:site/machine-id/bot/:name/token`<br><br>**Logic**<br>Use `Service.ListResourcesWithFilter` to retrieve a paginated list of `token` resources which have a role of "Bot" and `bot_name` matching the bot being queried.<br><br>**Performance**<br>Pagination is not required, so pages will be retrieved one after the other until the end of the list. It is not anticipated that any bot will have more than 20 join methods. |
| Update roles, traits, config and lock state     | `PUT /v1/webapi/sites/:site/machine-id/bot/:name`<br><br>**Logic**<br>Not all items of data are saved to the same resource. As such, it's important to ensure the update happens atomically and rolled back on failure.                                                                                                                                                                                                                               |
|                                                 |                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| **Existing**                                    |                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| Fetch a bot by name, including roles and traits | `GET /v1/webapi/sites/:site/machine-id/bot/:name`                                                                                                                                                                                                                                                                                                                                                                                                     |
| Delete a bot                                    | `DELETE /v1/webapi/sites/:site/machine-id/bot/:name`                                                                                                                                                                                                                                                                                                                                                                                                  |
| Fetch active instances for a bot by name        | `GET /v1/webapi/sites/:site/machine-id/bot-instance?search=:bot-name&page_size=20&page=/bot-instance/:bot_name/:instance_id`                                                                                                                                                                                                                                                                                                                          |

#### UI

Implementation of the UI will make heavy use of existing shared components. No new design components will be contributed.

#### Feature Flag

A feature flag should be used to allow partial features to be build and merged without worrying about incomplete features being released to customers.
#### Tasks

A rough breakdown of tasks with the goal of delivering implementation items in manageable chunks without requiring large PRs and time consuming reviews.

1. Enable deeplinking to Join Token view/edit page
2. Endpoint `GET /v1/webapi/sites/:site/machine-id/bot/:name/token`
3. Endpoint `PUT /v1/webapi/sites/:site/machine-id/bot/:name`
4. UI panel component
5. Bot info panel
6. Join tokens panel
7. Roles and traits panels
8. Active instances panel
9. Delete bot operation
10. Lock bot operation
11. Edit operation (inc roles, traits and max session duration)
### Wishlist features

#### Recent instances (historic)
Similar to the active instance list, except show instances whose credentials have recently expired (in the last 24 hours).
![[0215-feature-recent-intances.png]]
#### Audit log
Filtered to show only the bot being viewed. Needs to filter the log in a performant way, and likely only available to customers using Athena.
![[0215-feature-audit-log.png]]
#### Access Graph
Likely a simplified version of the Access Graph focused on the bot and without the ability to explore other parts of the graph.
![[0215-feature-access-graph.png]]

#### Session recordings
Bot sessions are non-interactive, so recordings are not possible in many cases. Session in this list may show command input and output, but aren't re-playable.
![[0215-feature-session-recordings.png]]

#### Activity visualisation
A minimalist representation of a bot activity over various time frames. Authentication records as well as heartbeats could be used to provide the data. Otherwise, data from the Audit Log could be used where that data is retrievable in a performant way (i.e. customers using Athena).
![[0215-feature-activity-timeseries.png]]