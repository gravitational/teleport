---
authors: Jan Kaczmarkiewicz (jan.kaczmarkiewicz@goteleport.com)
state: draft
---

# RFD 83 - Prevent user role self-lock

## Required Approvers

- Engineering: @jimbishopp
- Product: @xinding33

## What

Users can change the permission in a way that can cause no one to be able to change it further.

Currently, there are multiple ways for users to lock themselves:

| initial state                            | operation                                                                                   | result |
| ---------------------------------------- | ------------------------------------------------------------------------------------------- | :----: |
| _admin_ and _regular users_              | _admin_ deletes himself an `editor` role                                                    |   🔒   |
| _admin_ and _regular users_              | _admin_ loses access to own account                                                         |   🔒   |
| _admin_, _SSO admin_ and _regular users_ | _SSO admin_ removes the `editor` role from _admin_ and SSO connector. _SSO admin_ logs out. |   🔒   |
| _SSO admin_ and _regular users_          | Someone on the SSO side removes _SSO admin_ user or team/group containing _SSO admins_      |   🔒   |

Cases handled currently:

| initial state               | operation                                                                          | result |
| --------------------------- | ---------------------------------------------------------------------------------- | :----: |
| _admin_ and _regular users_ | _admin_ tries to delete his account. Operation is not permitted, an error is shown |   🔓   |

## Terminology

- _admin_: local user with `editor` role
- _SSO admin_: user with `editor` role added via SSO connector (eg. GitHub)
- _regular user_: user without `editor` role
- 🔒: No one can edit roles. Deadlock
- 🔓: There is still an active user with an editor role

## Why(TODO)

We want to prevent that since this is bad for user experience and it requires manual work on our side since we need to add a manually new editor user and communicate with teleport consumers.

## Details

> Context: SSO users
>
> Teleport has this feature called SSO auth. It allows to addition SSO connector to a cluster and maps SSO users/user groups to teleport roles. Currently, it works this way:
>
> 1. Authorized teleport users can add SSO connector (eg. GitHub SSO https://goteleport.com/docs/setup/admin/github-sso)
> 2. GitHub users that are members of a particular organization and the team can log in as teleport users with a defined role.
> 3. Users will show up in the users' list in UI but they will disappear when their cert expires (even when we remove the SSO connector).
> 4. SSO users can be `editors`.

The solution here could be to introduce a new rule:

- There should be at least 2 local _admins_

This will ensure that:

- When one of _admins_ loses access to the account the second one will be functional
- Users won't be able to delete _admin_ user if there are only 2 _admins_. They would have to add a third one and then they can delete it.
- Users won't be able to unassign the `editor` role from _admin_ user if there are only 2 _admins_. They would have to add a third and then they can unassign.
- Role system lock will be independent of connector removal or SSO changes: team deletion/user deletion.

Should we enforce this rule when a user is setting up its cluster? What about existing users? I think there are two approaches here:

- they would have to create immediately a second _admin_ user (if they have only one currently). They cannot use a cluster if they don't.
- would be to just warn them that adding a second _admin_ user is very important. If they do that warning will disappear and this will mean the rule is now fulfilled.

Let's compare those two:

|      | required on start                                                                               | optional on start                                                                                |
| ---- | ----------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| pros | user is unable to self-lock from the start if account is lost                                   | better onboarding experience since there are less steps nessessery to configure and use teleport |
| cons | worse onboarding experience since there are more steps nessessery to configure and use teleport | user is able to self-lock if account is lost                                                     |

In my opinion `optional on start` variant is better here since `teleport` should be as easy as possible to setup and later on when cluster will be used by more people having second admin would have more sense.

### UI and behavior changes

To introduce this change we need to communicate it to prevent confusion.

The first thing user should see is some indicator that there is recommended action to perform. It could be:

#### WebUI

1. ⚠️ icon in nav section + warning on users page:

   - icon:

   ```text
   👥  Team ⚠️       ⬎
       👥 Users ⚠️
       🔑 Roles
       ...
   ```

   - warning on /users page:

   ```text
   Users                                   [Create new user]
   ┌-------------------------------------------------------┐
   │   <Info why it is nessesery to add second user with   │
   │   `editor` role>                                      │
   └-------------------------------------------------------┘
   ... (Table of users)
   ```

2. warning on every page (eg. /cluster/{cluster}/nodes):
   ```text
   Servers
   ┌-------------------------------------------------------┐
   │   <Info why it is nessesery to add second user with   │
   │   `editor` role + link to /users page>                │
   └-------------------------------------------------------┘
   ... (Table of servers)
   ```

I think the `1.` option would be more direct and less distracting since this information is scoped in user managment.
This warning will be visible only for existing `editor` user.

#### CLI
