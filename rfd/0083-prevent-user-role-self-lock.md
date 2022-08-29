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

Actions that can cause this state:

- user
  - removal
  - role removal
- role
  - edit
- SSO
  - connector removal
  - user/group removal

Those actions will cause lock when:

| action                 | initial state                                 | change                                                                                 |
| ---------------------- | --------------------------------------------- | -------------------------------------------------------------------------------------- |
| user removal           | _admin_, user with user deletion capabilities | user with user deletion capabilities deletes _admin_                                   |
| user role removal      | _admin_                                       | _admin_ removes himself a role that grants him user/role editing capabilities          |
| role edit              | _admin_                                       | _admin_ edits own role to remove user/role editing capabilities                        |
| SSO connector removal  | _SSO admin_ (no _admin_)                      | _SSO admin_ removes SSO connector. _SSO admin_ logs out.                               |
| SSO user/group removal | _SSO admin_ (no _admin_)                      | Someone on the SSO side removes _SSO admin_ user or team/group containing _SSO admins_ |

Teleport already protects against a similar case in the following scenario:

| action       | initial state | change                           |
| ------------ | ------------- | -------------------------------- |
| role removal | _admin_       | _admin_ tries to remove own role |

Error message is diplayed: _failed to delete role that still in use by a user. Check system server logs for more details_.

There is one more case when user can lock himself out:

| initial state | change                                                                                  |
| ------------- | --------------------------------------------------------------------------------------- |
| _admin_       | admin looses acess to own account (eg. he stores credenials and recovery codes locally) |

see #TODO for details.

## Terminology

- _admin_: local user with user/role editing capabilities
- _SSO admin_: user with user/role editing capabilities added via SSO connector (eg. GitHub)

## Why

When a user locks themselves are unable to further manage the teleport cluster. We want to prevent that since this is bad for the user experience and prevents them from using the product.

Further, for our Cloud offering we don't perform account resets as a policy. The only recurse for a customer is to delete their cluster & deploy a new one. This inevitably leads to data-loss.

On-prem customers could work around this "locked out" state by manually instrumenting the auth process using tctl.

## Details

> Context: SSO users
>
> Teleport has this feature called SSO auth. It allows for the addition of an SSO connector to a cluster and maps SSO users/user groups to teleport roles. Currently, it works this way:
>
> 1. Authorized teleport users can add SSO connector (eg. GitHub SSO https://goteleport.com/docs/setup/admin/github-sso)
> 2. GitHub users that are members of a particular organization and the team can log in as teleport users with a defined role.
> 3. Users will show up in the users' list in UI but they will disappear when their cert expires (even when we remove the SSO connector).
> 4. SSO users can have full access.

The solution here could be to introduce a new rule:

- There should be at least 1 local _admin_

This will ensure that:

- Users won't be able to delete _admin_ user if there is only one _admin_. They would have to add a second one and then they can delete it.
- Users won't be able to unassign the user/role editing capabilities from _admin_ user if there are only is only one _admin_. They would have to add a second ond and then they can unassign.
- locking will be impossible on SSO side since there will be _admin_

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
┌---------------------------------------------------------┐
│   <Info why it is necessary to add a second user with   │
│   `editor` role>                                        │
└---------------------------------------------------------┘
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
