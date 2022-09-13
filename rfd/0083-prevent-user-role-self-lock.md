---
authors: Jan Kaczmarkiewicz (jan.kaczmarkiewicz@goteleport.com)
state: draft
---

# RFD 83 - Prevent user role self-lock

## Required Approvers

- Engineering: @jimbishopp
- Product: @xinding33

## What

Users can change their permissions in a way that leaves them incapable of making further changes to the cluster - in effect, they lock themselves out.

Actions that can cause this state:

- user
  - removal
  - role unassigment
- role
  - edit
- SSO
  - connector removal
  - user/group removal

Those actions will cause a lock when:

| action                 | initial state                                     | change                                                                                 |
| ---------------------- | ------------------------------------------------- | -------------------------------------------------------------------------------------- |
| user removal           | _admin_, the user with user deletion capabilities | user with user deletion capabilities deletes _admin_                                   |
| user role removal      | _admin_                                           | _admin_ removes himself from a role that grants him user/role editing capabilities     |
| role edit              | _admin_                                           | _admin_ edits own role to remove user/role editing capabilities                        |
| SSO connector removal  | _SSO admin_                                       | _SSO admin_ removes SSO connector. _SSO admin_ logs out.                               |
| SSO user/group removal | _SSO admin_                                       | Someone on the SSO side removes _SSO admin_ user or team/group containing _SSO admins_ |

Teleport already protects against a similar case in the following scenario:

| action       | initial state | change                           |
| ------------ | ------------- | -------------------------------- |
| role removal | _admin_       | _admin_ tries to remove own role |

The error message is displayed: _failed to delete role that still in use by a user. Check system server logs for more details_.

There is one more case when a user can lock himself out:

| initial state | change                                                                                    |
| ------------- | ----------------------------------------------------------------------------------------- |
| _admin_       | _admin_ loses acess to own account (eg. he stores credentials and recovery codes locally) |

However, requiring a second admin to guard this situation is not good for user experience. Existing and new clusters will have to add a second _admin_.

## Terminology

- _admin_: local user with user/role editing capabilities
- _SSO admin_: user with user/role editing capabilities added via SSO connector (eg. GitHub)

## Why

When a user locks themselves out, they are unable to further manage the teleport cluster. We want to prevent that since this is bad for the user experience and prevents them from using the product.

Further, for our Cloud offering, we don't perform account resets as a policy. The only recourse for a customer is to delete their cluster & deploy a new one. This inevitably leads to data loss.

On-prem customers could work around this "locked out" state by running `tctl` locally on the auth server to create a new admin user.

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
- Users won't be able to unassign the user/role editing capabilities from _admin_ user if there are only is only one _admin_. They would have to add a second one and then they can unassign.
- Locking will be impossible on the SSO side since there will be always _admin_ on our side.

Now we need to define what _admin_ means in our system. W can take 2 approaches:

1. User with an immutable `editor` role is _admin_.
2. User with a role that grants user/role editing capabilities is _admin_.

From the user's perspective, the first approach is simpler. The error message saying: "You can't delete the last user with `editor` role." will be easier to understand than "You can't delete the last user with: list of role management capabilities".

The other benefit is that we wouldn't need to define validation for editing roles (since the `editor` role would be immutable).

The drawbacks here are:

- we force users to have the `editor` role
- complicated migration in case of updating cluster. It is still possible to delete/edit default roles so not all existing clusters will have default roles in the initial form. Maybe the `editor` role is used as a role with a different meaning than the initial one? We can't override existing roles. So the solution would be to add 3 new roles (instead of editing/re-adding: editor, auditor, access). This will ensure we don't break anything. But the problem will be users would have to assign manually a new _admin_ role to some users.

I think `2.` approach is better since migration cost outweighs the benefits of the pros.

### UI and behavior changes

We need to change the UI to reflect the new policy. We need to add validation to all actions that can cause lock (in `WebUI` and `tctl`). The error messages should inform the user why this action is not allowed.

No changes are required in the `tsh` code.
