---
authors: Jan Kaczmarkiewicz (jan.kaczmarkiewicz@goteleport.com)
state: draft
---

# RFD 83 - Prevent user role self lock

## Required Approvers

- Engineering: @jimbishopp
- Product: @xinding33

## What

User can change permission in a way that can cause noone to be able to change it further.

Currently there are multiple ways for users to lock themselves:

| initial state                            | operation                                                                               | result |
| ---------------------------------------- | --------------------------------------------------------------------------------------- | :----: |
| _admin_ and _regular users_              | _admin_ deletes himself a `editor` role                                                 |   🔒   |
| _admin_ and _regular users_              | _admin_ looses access to own account                                                    |   🔒   |
| _admin_, _SSO admin_ and _regular users_ | _SSO admin_ removes `editor` role from _admin_ and SSO connector. _SSO admin_ logs out. |   🔒   |
| _SSO admin_ and _regular users_          | Someone on SSO side removes _SSO admin_ user or team/group containig _SSO admins_       |   🔒   |

Cases handled currently:

| initial state               | operation                                                                       | result |
| --------------------------- | ------------------------------------------------------------------------------- | :----: |
| _admin_ and _regular users_ | _admin_ tries to delete own account. Operation is not permitted, error is shown |   🔓   |

## Why(TODO)

We want to prevent that since this is bad for user experience and it requires manual work on our side since we need to add manualy new editor user and communicate with clinet.

## Terminology

- _admin_: local user with `editor` role
- _SSO admin_: user with `editor` role added via SSO connector (eg. GitHub)
- _regular user_: user without `editor` role
- 🔒: No one can edit roles. Deadlock
- 🔓: There is still active user with editor role

## Details

> Context: SSO users
>
> Teleport has this feature called SSO auth. It allows to add SSO connector to cluster, and map SSO users/user groups to teleport roles. Currently it works this way:
>
> 1. Authorized teleport user can add SSO connector (eg. GitHub SSO https://goteleport.com/docs/setup/admin/github-sso)
> 2. GitHub users that are members of particaular organization and team can login as teleport users with defined role.
> 3. User will show up in users list in UI but they will desapear when their cert expires (even when we remove SSO connector).
> 4. SSO users can be `editors`.

The solution here could be to introduce new rule:

- There should be at least 2 local _admins_

This will ensure that:

- When one of _admins_ looses access to accout the second one will be functional
- Users won't be able to delete _admin_ user if there are only 2 _admins_. They would have to add third one and then they can delete.
- Users won't be able to unassign `editor` role from _admin_ user if there are only 2 _admins_. They would have to add third and then they can unassign.
- Users system lock will be independent from connector removal or SSO changes: team deletion / user deletion.

Should we enforce this rule when user is setuping its cluster? What about existing users? I think there are two approaches here:

- they would have to create immediately second _admin_ user (if they have only one currently). They cannot use cluser if they don't.
- would be to just warn them that adding second _admin_ user is very important. If they do that we can remove warning and this will mean the rule is now fullfilled.

Lets compare those two:

| required on start | optional on start |
| ----------------- | ----------------- | ---- |
|                   |                   | pros |
| ----------------- | ----------------- | ---  |
|                   |                   | cons |

### UI and behavior changes

To introduce this change we need to clearly communicate it preventing confussion
TODO
