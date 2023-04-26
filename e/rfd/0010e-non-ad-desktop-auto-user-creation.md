---
authors: Przemko Robakowski (przemko.robakowski@goteleport.com)
state: draft
---

# RFD 0010e - Automatic user creation for non-AD Desktop Access

## Required approvers

Engineering: @zmb3
Product: @klizhentas, @xinding33
Security: @reedloden, @jentfoo

## What

Proposes support for automatic user creation for non-AD Desktop Access.

## Why

Today, we require all users to exist on the Windows machine beforehand. This means that desktop access users have to
either
use a predefined set of Windows users for all their Teleport users, or build extra automation to provision Windows
users separately when onboarding new users.

Adding support for automatic user provisioning, similar to
[automatic Linux user provisioning](https://github.com/gravitational/teleport/blob/master/rfd/0057-automatic-user-provisioning.md),
will allow desktop access users to not worry about preconfiguring each individual user, which will simplify user
on-/offboarding and give users the ability to control permissions via IdP traits.

## Scope

The RFD will cover only non-AD setup, managing users in Active Directory is out of scope.

## High-level flow

Let's first define the high-level flow of how automatic user creation will work:

- User starts desktop session selecting login in UI
- Teleport Authentication Package (AP), that is installed on Windows machine, verifies certificate and retrieves
  username
- If the user doesn't exist on the Windows machine, the AP creates it.
- AP activates users if needed and syncs user's groups
- When session ends AP disables user

Now let's dive into details.

## Details

### Role changes

The role resource will be updated to include additional options/fields:

- `options.create_desktop_user` will indicate whether Teleport should attempt to
  create a Windows user for the connecting Teleport user.
- `allow.desktop_groups` will contain group names the created user should
  be member of. Will support IdP trait templating.

```yaml
kind: "role"
version: "v5"
metadata:
  name: "example"
spec:
  options:
    create_desktop_user: true
  allow:
    desktop_groups: [ "reader", "writer", "{{external.desktop_groups}}" ]
    windows_desktop_logins: ['DBAdmin']
    windows_desktop_labels:
      'env': ['staging', 'test']
```

If user has multiple roles with fields above we will take union of them the same way we do it in server access.
If at least one role matches the desktop (by means of `windows_desktop_labels`) but does not
include `create_desktop_user: true`, automatic user creation will be disabled. Roles that do not match the desktop will
not be checked. This matches the behavior in server access.

To create user, role has to include requested username in `windows_desktop_logins`. This matches the behavior in server
access.

### Certificate changes

A new extension will be added to certificate used for Windows authentication with OID `1.3.9999.2.16`. It will encode
whether a user should be created and which groups they should be part of. This extension will be parsed by the Authentication
Package. It will be encoded as JSON string.

### User creation

If the login requested by user is missing on Windows machine, and the certificate includes the new extension,
Authentication Package will create it using
[NetUserAdd](https://learn.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netuseradd) API.
All users created that way will be added to `Teleport Users` group for bookkeeping purposes. `Teleport Users` group
will be created upon first start.
User will never be deleted by Teleport to preserve access to user profile on next login - when you delete user
their profile is preserved but non-administrator users can't access it even if user with the same username is added
later as the SID is different. This is similar to the way users are managed in DB access.

### Groups synchronization

If the username is managed by Teleport (it's either brand new or is a member of the `Teleport Users` group) then groups
will be synchronized to
the set requested in the certificate before each session: user will be removed from all groups that are not requested in
the certificate and
will be added to all groups that are requested.
Windows enforces that there can only ever be one session for a given username, so there's no concern of synchronization
problems being caused by multiple sessions of the same username with different group permissions.

Missing groups will be created using
[NetLocalGroupAdd](https://learn.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netlocalgroupadd) API.
Groups will never be deleted by Teleport. This is similar to the way groups are managed in Server access.

### User activation and disabling

If the user is disabled it can't be used for login. That means user can't be exploited through regular RDP connection
or local login.
Change of active status can be done using
[NetUserSetInfo](https://learn.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netusersetinfo) API with
level 1008 (flags).
At the system start, when Authentication Package is initialized, it will disable all users managed by Teleport.
During login, it will activate requested user and then, when sessions ends (LSA calls `LsaApLogonTerminated ` method),
it will deactivate the user again.

### Synchronization

All login requests are going through
the single [LSA](https://en.wikipedia.org/wiki/Local_Security_Authority_Subsystem_Service) process, so the Go `sync`
package will
be used to synchronize parallel login requests.