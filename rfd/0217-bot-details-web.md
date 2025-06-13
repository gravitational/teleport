---
authors: Nick Marais (nicholas.marais@goteleport.com)
state: draft
---
# RFD0217 - Bot Details (web)

## Required Approvers

* Engineering: @strideynet
* Product: @thedevelopnik && @samratambadekar

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

As a member of the **Incident Response team**,
I would like to **lock a bot** and all active instances,
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

As a member of the **DevOps/Development team**, who has just set up an MWI agent (e.g. GitHub Actions),
I would like to get **realtime feedback** that my new **agent/s are enrolling correctly**,
So that I can feel confident that the agents will work going forwards.

### UX

![](assets/0217-iteration-1.png)

#### Bot info and config

Shows basic details and configuration. All items are readonly. Date/time items have a hover state which shows a tooltip with the full date and time.

![](assets/0217-feature-info.png)

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

![](assets/0217-feature-join-tokens.png)
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
Provides full lists of roles and traits (well-known and custom). Edit operations are provided for each for convenience, which open the page-wide edit modal with all editable fields available.

![](assets/0217-feature-roles-traits.png)
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
Lists the most recent (max 10) instances for the bot, ordered most recent first. A refresh action reloads the data for this panel only, and is provided to make monitoring instance activity easier. A "see more" action navigates to the bot instances page with a pre-populated search filter on the bot's name - this is an imperfect filter as it's a contains-text filter across all fields.

![](assets/0217-feature-active-instances.png)

> [!TODO]
> Add an explanation of the approach to using an in-memory cache to store and maintain an index on an instance's most recent activity timestamp. This allows instance records to be retrieved in order of most recent activity first (or last, although not required).

#### Edit roles, traits and max session duration (`max_session_ttl`)
Shows a dialog where the user can add and/or remove assigned roles, add and/or remove traits (well-known or custom), and edit the configured max session duration in the form `43200s`, `30m` or `3h`. Allows all changes to be made in a single atomic transaction.

![](assets/0217-feature-edit.png)

#### Delete bot
Deletes the bot after confirmation. Shows a loading indicator during the call to the api. On success, navigates to the bots list (`/web/bots`). On error, shows a message within the confirmation dialog.

![](assets/0217-feature-delete-bot.png)
#### Lock bot
Locks the bot after confirmation. Shows a loading indicator during the call to the api. On success, removes the dialog but remains on the bot detail page. On error, shows a message within the confirmation dialog.

![](assets/0217-feature-lock-bot.png)

Once a bot is locked, to unlock it requires navigating to **Identity Governance > Session & Identity Locks**, finding the lock in question and removing it. Deleting a lock requires it's UUID, which wont be know on this page.
### Implementation

#### Data fetching and caching

In order to keep the implementation modular, each logical section of the page fetches its own data, shows loading status and manages error states. This increases flexibility for future iterations while reducing the likelihood of introducing regression issues. Data caching behaviour can be tailored to each area of data independently.

#### Web APIs

##### `GET /v1/webapi/sites/:site/machine-id/bot/:name`

Fetch a bot by name, including roles and traits. Endpoint exists and will be reused. Additional fields for lock state will be added to the Bot resource and populated from the underlying User resource.

``` protobuf
message BotStatus {
  // IsLocked indicates if the bot user is locked
  bool IsLocked = 4
  // LockedMessage contains the message to display when the bot user is locked
  string LockedMessage = 5
  // LockedTime contains the time when bot user was locked
  google.protobuf.Timestamp LockedTime = 6
  // LockExpires contains the time this lock will expire
  google.protobuf.Timestamp LockExpires = 7
}
```

##### `GET /v1/webapi/tokens?filter_role=Bot&filter_bot_name=:name`

Fetch join tokens linked to a bot by name.

**Approach**
Existing endpoint with additional filters added. Filter params will be added; `filter_role` and `filter_bot_name`

``` protobuf
// GetTokensRequest is used to request a list of tokens. Filtering by role and bot_name are supported
message GetTokensRequest {
    // FilterRole allows filtering the returned items by the token's role (e.g. 'static', 'node', 'bot', etc)
    string FilterRole = 1 [(gogoproto.jsontag) = "filter_role"];
    // FilterBotName allows filtering the returned items by the name of the associated bot. This is a no-op unless FilterRole is 'bot'.
    string FilterBotName = 2 [(gogoproto.jsontag) = "filter_bot_name"];
}

service AuthService {
  // GetTokens retrieves all tokens.
  rpc GetTokens(GetTokensRequest) returns (types.ProvisionTokenV2List);
}
```
_api/proto/teleport/legacy/client/proto/authservice.proto_

**Performance**
The existing endpoint does not support pagination. It is not anticipated that use of the endpoint for the Bot Details page will increase usage or load significantly, as such, the endpoint will be used as-is. A bot is likely to have fewer than 20 associated join methods.

##### `GET /v1/webapi/sites/:site/machine-id/bot-instance?search=:bot-name&page_size=20&page=/bot-instance/:bot_name/:instance_id`

Fetch active instances for a bot by name. Endpoint exists.

##### `PUT /v1/webapi/sites/:site/machine-id/bot/:name`

Update roles, traits and config (`max_session_ttl`).

**Approach**
Existing endpoint with additional capabilities added. Currently only updating roles is supported. Support for updating traits and `max_session_ttl` are already supported by the underlying RPC and will be exposed by this endpoint. Not all items of data are saved to the same resource. As such, it's important to ensure the update happens atomically and rolled back on failure.

##### `DELETE /v1/webapi/sites/:site/machine-id/bot/:name`

Delete a bot. Existing endpoint.

##### `PUT /v1/webapi/sites/:site/locks`

Creates a new lock. Endpoint exists and will be used as-is to create a lock for a bot by name including a message and TTL.

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
![](assets/0217-feature-recent-intances.png)
#### Audit log
Filtered to show only the bot being viewed. Needs to filter the log in a performant way, and likely only available to customers using Athena.
![](assets/0217-feature-audit-log.png)
#### Access Graph
Likely a simplified version of the Access Graph focused on the bot and without the ability to explore other parts of the graph.
![](assets/0217-feature-access-graph.png)

#### Session recordings
Bot sessions are non-interactive, so recordings are not possible in many cases. Session in this list may show command input and output, but aren't re-playable.
![](assets/0217-feature-session-recordings.png)

#### Activity visualisation
A minimalist representation of a bot activity over various time frames. Authentication records as well as heartbeats could be used to provide the data. Otherwise, data from the Audit Log could be used where that data is retrievable in a performant way (i.e. customers using Athena).
![](assets/0217-feature-activity-timeseries.png)
