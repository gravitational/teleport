---
authors: Jan Kaczmarkiewicz (jan.kaczmarkiewicz@goteleport.com)
state: draft
---

# RFD 86 - Prevent user role self-lock

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

Further, for our Cloud offering, we don't perform account resets as a policy (as per https://gravitational.slab.com/posts/engaging-with-the-cloud-team-k1b83cit). The only recourse for a customer is to delete their cluster & deploy a new one. This inevitably leads to data loss.

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

- There should be at least one _admin_ user.

This will ensure that:

- Users won't be able to delete _admin_ user if there is only one _admin_. They would have to add a second one and then they can delete it.
- Users won't be able to unassign the user/role editing capabilities from _admin_ user if there are only is only one _admin_. They would have to add a second one and then they can unassign.
- Locking will be impossible on the SSO side since there will be always _admin_ on our side.

### UI and behavior changes

We need to add validation to all actions that can cause lock (in `WebUI` and `tctl`). The error messages should inform the user why this action is not allowed.

No changes are required in the `tsh` code.

### Alternatives Considered

#### immutable roles

We would automatically add one or more immutable roles. They cannot be added, edited, or deleted by cluster users. There should be at least one immutable role with user/role editing capabilities. This role should be assigned to at least one user.

The error message saying: "You can't delete the last user with the `editor-like` role." will be easier to understand than "You can't delete the last user with: `list of role management capabilities`".

The other benefit is that we wouldn't need to define validation for editing roles (since the `editor-like` role would be immutable).

The drawbacks here are:

- we force users to have the `editor-like` role
- complicated migration in case of updating cluster. It is still possible to delete/edit default roles so not all existing clusters will have default roles in the initial form. Maybe the `editor-like` role is used as a role with a different meaning than the initial one. We can't override existing roles. So the solution would be to add 3 new roles (instead of editing/re-adding: editor, auditor, access). This will ensure we don't break anything. The problem will be users would have to assign manually a new _admin_ role to some users.

#### root-like user

Automatically add an immutable user with full access. This user would be created during the cluster creation/upgrade. This user would be assigned to the `root` immutable role (full access).

We could add a `root` user automatically using a username and random password generation. This could be done by running some procedure when a cluster is updated or initially bootstrapped. The authorized user can then reset authentication and set a known password or another form of auth. This is optional for a cluster to function but is required to prevent the lock problem. We could add the warning that it is recommended action.

Customers will mostly use passwords as auth since the `root` user will be not a personal account. Shared passwords are much more likely to be misused, their passwords tend to remain unchanged for extended periods, and often leak when employees change jobs. Also, since they are nobody's personal responsibility and sort of common knowledge among peers, they tend to not get the same amount of diligence as personal accounts and are often emailed or written down in notes, files, and password managers. Hackers could take advantage of that to stole it and gain access to the cluster.

Also, some customers may don't want extra user.
