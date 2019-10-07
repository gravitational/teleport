# Teleport User

TODO: Intro/Overview of Teleport User

## Adding and Deleting Users

This section covers internal user identities, i.e. user accounts created and
stored in Teleport's internal storage. Most production users of Teleport use
_external_ users via [Github](#github-oauth-20) or [Okta](ssh_okta) or any
other SSO provider (Teleport Enterprise supports any SAML or OIDC compliant
identity provider).

A user identity in Teleport exists in the scope of a cluster. The member nodes
of a cluster have multiple OS users on them. A Teleport administrator creates Teleport user accounts and maps them to the allowed OS user logins they can use.

Let's look at this table:

|Teleport User | Allowed OS Logins | Description
|------------------|---------------|-----------------------------
|joe    | joe,root | Teleport user 'joe' can login into member nodes as OS user 'joe' or 'root'
|bob    | bob      | Teleport user 'bob' can login into member nodes only as OS user 'bob'
|ross   |          | If no OS login is specified, it defaults to the same name as the Teleport user.

To add a new user to Teleport, you have to use the `tctl` tool on the same node where
the auth server is running, i.e. `teleport` was started with `--roles=auth`.

```bsh
$ tctl users add joe joe,root
```

Teleport generates an auto-expiring token (with a TTL of 1 hour) and prints the token
URL which must be used before the TTL expires.

```bsh
Signup token has been created. Share this URL with the user:
https://<proxy>:3080/web/newuser/xxxxxxxxxxxx

NOTE: make sure the <proxy> host is accessible.
```

The user completes registration by visiting this URL in their web browser, picking a password and
configuring the 2nd factor authentication. If the credentials are correct, the auth
server generates and signs a new certificate and the client stores this key and will use
it for subsequent logins. The key will automatically expire after 12 hours by default after which
the user will need to log back in with her credentials. This TTL can be configured to a different value. Once authenticated, the account will become visible via `tctl`:

```bsh
$ tctl users ls

User           Allowed Logins
----           --------------
admin          admin,root
ross           ross
joe            joe,root
```

Joe would then use the `tsh` client tool to log in to member node "luna" via
bastion "work" _as root_:

```yaml
$ tsh --proxy=work --user=joe root@luna
```

To delete this user:

```yaml
$ tctl users rm joe
```

## Editing Users

Users entries can be manipulated using the generic [resource commands](#resources)
via `tctl`. For example, to see the full list of user records, an administrator
can execute:

```yaml
$ tctl get users
```

To edit the user "joe":

```yaml
# dump the user definition into a file:
$ tctl get user/joe > joe.yaml
# ... edit the contents of joe.yaml

# update the user record:
$ tctl create -f joe.yaml
```

Some fields in the user record are reserved for internal use. Some of them
will be finalized and documented in the future versions. Fields like
`is_locked` or `traits/logins` can be used starting in version 2.3

Every User must be associated with one or mote OS-level users. When you create a SSH session on a Node, you will authenticated as one of the Node's during a login. This list is called "user mappings".


```bash
[teleport@grav-00 ~]$ export tuser=petra # set $user to anything you want
[teleport@grav-00 ~]$ tctl users add $tuser teleport
Signup token has been created and is valid for 1 hours. Share this URL with the user:
https://grav-00:3080/web/newuser/3a8e9fb6a5093a47b547c0f32e3a98d4

NOTE: Make sure grav-00:3080 points at a Teleport proxy which users can access.
```

By default a new Teleport user will be assigned a mapping to an OS-user of the same name. For example, if we didn't specify the last argument `teleport`,the teleport user would expect to log in as an OS-user called `petra`. Now have a mapping between `petra` and `teleport`. If you want to map to a different OS-user you can run `tctl users add <teleport-user> <os-user>`