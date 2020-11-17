---
authors: Alexander Klizhentas (sasha@gravitational.com), Ben Arent (ben@gravitational.com)
state: discussion
---

# RFD 2 - Advanced Session Control

## What

Teleport provides secure access to infrastructure, but security teams and FedRAMP requires
greater control over a session once it's started. Teleport already provides limits
regarding concurrent session control. This helps companies obtain the NIST AC-10
Concurrent Session Control control.

This RFD provides a user locking mechanism to stop a user from continuing to access
a system. This RFD will focus on how to disconnect and disable a user:

+ Teleport should disconnect and terminate any existing active sessions for the user.
+ Teleport should not allow creation of new sessions.

## Why

We've had several requests for advanced session control. These requests can be grouped
into these categories.

+ Keep a closer eye on contractor activity: Customers
+ Locking out the team during maintenance windows.
+ Terminating access for an employee before the TTL of Teleport's cert.
+ Obtaining FedRAMP NIST Control

### AC-17 Access Control

> AC-17 - ACCESS CONTROL - REMOTE ACCESS
> The organization provides the capability to expeditiously disconnect or disable
> remote access to the information system within [Assignment: organization-defined time period].
> Supplemental Guidance: This control enhancement requires organizations to have the capability
> to rapidly disconnect current  users remotely accessing the information system
> and/or disable further remote access.


## Details

### Listing active sessions
The `tctl` command needs to be updated to support listing all active sessions and
print the users associated with them.

```bash
$ tctl sessions ls

Session ID   User(s)                           Node                 Created
-----------  --------                          -----                --------
b4971e71     foo@example.com                   server01 [10.0.0.1]  01/25/2019 14:22:31
e8bce547     bar@example.com                   server01 [10.0.0.1]  01/20/2019 08:55:00
52148bec     foo@example.com, bar@example.com  server02 [10.0.0.2]  01/25/2019 08:12:31
4bae64c2     bar@example.com                   server03 [10.0.0.3]  01/25/2019 03:22:11
```

### Generic Locking
The `tctl` command needs to be updated to support locking and unlocking and updated
to list locks.

The command to lock a user will look like the following (where message and expires
is optional).

```bash
$ tctl lock --user=foo@example.com --message="Suspicious activity." --expires-in=0s
#User foo@example.com locked permanently. Use “tctl rm locks/id” to unlock

$ tctl lock --role=developers --message="Cluster maintenance." --expires=”Monday, 21 September 2019”
#Role developers locked.

$ tctl lock --cluster=”example.com” --message=”Time of day crew access” --expires-in=100h
# All existing sessions will be terminated, establishment of new sessions will
# be restricted, and credentials will not be re-issued.
```

The command to unlock a user:

```bash
$ tctl unlock --user=foo@example.com
# User foo@example.com unlocked.
```

User will be able to create new sessions and request credentials be re-issued.

The existing `tctl users ls` command will be used to show user lock status:

```bash
$ tctl locks ls

User              Roles   Locked
----------------  ------  --------------------------
foo@example.com   admin   Until 01/25/2019 14:22:31
```

## Implementation

The `tctl users lock` and `tctl users unlock` commands will update `services.UserSpecV2`
for the given user setting the Status fields to mark the user is locked or unlocked
respectively.

In `srv.ServerContext`, there are periodic checks for disconnecting idle clients
and expired certificates. This method will also watch `services.UserSpecV2` for
changes. Upon a change in the status field, the active session will be terminated.

In `srv.AuthHandlers`, after the certificate has been validated, the user status
is checked. If the user is locked, the connection will be terminated.

In the `auth.AuthServer` during the login process, if a user for that name already
exists and is locked, don't Upsert a new user. This also prevents the issuing of
new certificates.
