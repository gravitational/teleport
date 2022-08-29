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

- There should be at least _admin_

This will ensure that:

- Users won't be able to delete _admin_ user if there is only one _admin_. They would have to add a second one and then they can delete it.
- Users won't be able to unassign the user/role editing capabilities from _admin_ user if there are only is only one _admin_. They would have to add a second ond and then they can unassign.
- Locking will be impossible on SSO side since there will be always _admin_ on our side.

Now we need to define what _admin_ mean in our system. W can take 2 approaches:

1. User with immutable `editor` role is _admin_.
2. User with role that grants user/role editing capabilities is _admin_.

From the user perspective, the first approach is simpler. The error message saying: "You can't delete last user user with `editor` role." will be easier to understand than "You can't delete last user with: list of full user & role managment capabilities".

Also implementation would be slighty easier. We wouldn't need to define validation for editing roles (since `editor` role would be immutable).

The drowback here is that we force users to have `editor` role.

### UI and behavior changes

We need to change the UI to reflect the new policy. We need to add validation to all actions that can cause lock (in `WebUI` and `tctl`). The error messages should clearly inform user why this action is not allowed.

No changes required in `tsh` code.
