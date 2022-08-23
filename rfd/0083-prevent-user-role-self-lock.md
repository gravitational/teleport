---
authors: Jan Kaczmarkiewicz (jan.kaczmarkiewicz@goteleport.com)
state: draft
---

# RFD 83 - Prevent user role self-lock

## Required Approvers

- Engineering: @jimbishopp
- Product: @xinding33

## What

Users can change their permissions in a way that leaves them incapable of making further changes to the cluster - in effect they lock themselves out.

Currently, there are multiple ways to trigger this scenario:

| initial state                              | operation                                                                                   | result |
| ------------------------------------------ | ------------------------------------------------------------------------------------------- | :----: |
| 1 _admin_ and _regular users_              | _admin_ removes himself from an `editor` role                                               |   🔒   |
| 1 _admin_ and _regular users_              | _admin_ loses access to own account                                                         |   🔒   |
| 1 _admin_, _SSO admin_ and _regular users_ | _SSO admin_ removes the `editor` role from _admin_ and SSO connector. _SSO admin_ logs out. |   🔒   |
| _SSO admin_ and _regular users_            | Someone on the SSO side removes _SSO admin_ user or team/group containing _SSO admins_      |   🔒   |

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

## Why

When a user locks themselves are unable to further manage the teleport cluster. We want to prevent that since this is bad for the user experience and prevents them from using the product.

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

- When one of the _admins_ loses access to the account the second one will operate.
- Users won't be able to delete _admin_ user if there are only 2 _admins_. They would have to add a third one and then they can delete it.
- Users won't be able to unassign the `editor` role from _admin_ user if there are only 2 _admins_. They would have to add a third and then they can unassign.
- Role system lock will be independent of connector removal or SSO changes: team deletion/user deletion.

Should we enforce this rule when a user is setting up its cluster? What about existing users? I think there are two approaches here:

- they would have to create immediately a second _admin_ user (if they have only one currently). They cannot use a cluster if they don't.
- would be to just warn them that adding a second _admin_ user is very important. If they do that warning will disappear and this will mean the rule is now fulfilled.

Let's compare those two:

|      | required on start                                                                              | optional on start                                                                                |
| ---- | ---------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| pros | user is unable to self-lock from the start if an account is lost                               | better onboarding experience since there are fewer steps necessary to configure and use teleport |
| cons | worse onboarding experience since there are more steps necessary to configure and use teleport | user can self-lock if an account is lost                                                         |

In my opinion `optional on start` variant is better since `teleport` should be as easy as possible to set up and later on when the cluster will be used by more people having a second admin would make more sense.

### UI and behavior changes

To introduce this change we need to communicate it to prevent confusion.

The first thing user should see is some indicator that there are recommended actions to perform. It could be:

#### WebUI

##### Warning second user is missing

- ⚠️ icon in navigation:

```text
👥  Team ⚠️       ⬎
    👥 Users ⚠️
    🔑 Roles
    ...
```

- warning on `/users` page:

```text
Users                                   [Create new user]
┌-------------------------------------------------------┐
│   <Info why it is necessary to add a second user with   │
│   `editor` role>                                      │
└-------------------------------------------------------┘
... (Table of users)
```

> This will be visible only for the first _admin_.

##### Roles

When changing roles we should check if user change is not breaking the rule.

- disable `editor` chip when editing user roles
- disable delete _admin_ user action

In those cases, we should inform users why this action is not possible.

#### tsh

no changes required

#### tctl

##### Warning second user is missing

when using `tctl users ...` we should warn the user that adding a second _admin_ is highly recommended. This could also print example command: `tctl users add --roles=editor <name-of-editor>`. The message should be visible for the first _admin_ in the cluster and disappear after the second _admin_ is added.

##### Roles

when there is an attempt to break the rule program should display an error with an explanation of why this is not allowed (deleting one of two existing _admins_, removing the role `admin` from one of two existing _admins_)
