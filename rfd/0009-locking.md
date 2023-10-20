---
authors: Andrej Tokarčík (andrej@goteleport.com)
state: implemented
---

# RFD 9 - Locking

## What

This RFD provides a locking mechanism to restrict access to a Teleport
environment.  When such a lock is in force, all interactions matching
the lock's conditions are either terminated or prevented.

## Why

Security teams require greater control over _interactions_ (sessions,
connections, generation of certificates) in order to be able to:
+ lock out the team during maintenance windows,
+ terminate and disable further access for users, even if in possession of a valid Teleport certificate,
+ achieve FedRAMP/NIST compliance.

## Details

### `Lock` resource

A new resource named `Lock` with the following specification is introduced:

```proto
message LockSpecV2 {
    // Target describes the set of interactions that the lock applies to.
    LockTarget Target;

    // Message is the message displayed to locked-out users.
    string Message;

    // Expires if set specifies TTL for the lock.
    google.protobuf.Timestamp Expires;
}

// LockTarget lists the attributes of interactions to be disabled.
// The attributes are interpreted/matched qua simple names,
// with no support for wildcards or regular expressions.
message LockTarget {
    // User specifies the name of a Teleport user.
    string User;

    // Role specifies the name of an RBAC role known to the root cluster.
    // In remote clusters, this constraint is evaluated with both original
    // and mapped roles.
    string Role;

    // Login specifies the name of a local UNIX user.
    string Login;

    // Node specifies the name or UUID of a node.
    // A matching node is also prevented from heartbeating to the auth server.
    // DEPRECATED: use ServerID instead.
    string Node;

    // MFADevice specifies the UUID of a user MFA device.
    string MFADevice;

    // WindowsDesktop specifies the name of a Windows desktop.
    string WindowsDesktop string

    // AccessRequest specifies the UUID of an access request.
    AccessRequest string

    // Device is the device ID of a trusted device.
    // Requires Teleport Enterprise.
    Device               string

    // ServerID specifies the UUID of a Teleport server.
    ServerID             string
}
```

In most cases `LockTarget` should list just a single attribute.
(It would be preferable to define it as a `oneof`, however setting a custom
`jsontag` for `oneof` is [not supported by `gogoproto`](https://github.com/gogo/protobuf/issues/623).)

Note that `Lock` is not a singleton resource: when there are multiple
`Lock`s stored (and in force), it suffices for an interaction to be matched
by any one of the locks to be terminated/disabled.

#### Relation to `User.Status` and `LoginStatus`

There already exists a field of `User` resource that is used to represent a user lock in connection with failed Web UI login attempts. This `Status` field and its `LoginStatus` definition deal with limiting _authentication_ attempts.

The locks discussed in this RFD, however, implement a wide-ranging _authorization_ mechanism. It is not reasonable to impose a `Lock` in this sense (thus blocking the totality of a user's interactions, impacting even those already established based on successful authentication) in response to failed logins, even temporarily. Therefore, the two "locking" schemes should both be retained as they act on different levels (authn v. authz) and address complementary needs.

#### `tctl` support

`Lock` resources can be managed using `tctl [get|create|rm]`.  In this way,
it is possible to update or remove a lock after it has been created.

There will be a special `tctl lock` helper provided, to facilitate creation
of new `Lock`s, see Scenarios below.

### Propagation within a Teleport cluster

Instead of relying on the main cache system for lock propagation, every cluster component (Teleport process) will be initialized with its own local lock-specific watcher.

Such a `LockWatcher` will keep a single connection to the auth server for the purpose of monitoring the `Lock` resources. It will in turn allow derived watchers ("lock watcher subscriptions") which can be configured by a list of `LockTarget`s. It will internally record any connection/staleness issues while attempting to reestablish the connection to the auth server in the background.

`LockWatcher` will be parametrized by a duration that defines the maximum failure period after which the data are to be considered stale (defaulting to 5 minutes). If this tolerance interval is exceeded, interactions may be locked out preventively in accordance with the fallback mode described below.

### Fallback mode

If a local lock view becomes stale, there is a decision to be made about whether to rely on the last known locks. This decision strategy is encoded as a "locking mode".

When a transaction involves a user, the mode is inferred from the following option of the user's roles:

```
spec:
   options:
       # In strict mode, if the locks are not up to date, all matching interactions will be terminated.
       # In best_effort mode, the most recent view of the locks will remain to be used.
       lock: [strict|best_effort]
```

When none of the user's roles specify the mode or when there is no user involved, the mode is supplied from `AuthPreference.LockingMode` which configures a cluster-wide locking mode default. It defaults to `best_effort` to retain backward compatibility with HA deployments.

Note that if at least one of the inputs considered for the locking mode (user's roles, the cluster-wide default) is set to `strict`, the overall result is also `strict`. In this sense a single `strict` overrides any other occurrences of `best_effort`.

### Disable generating new certificates

No new user or host certificates matching a lock target should be generated
while the lock is in force.  An audit event should be emitted upon such
an attempt.

A new lock check will be added to the `generateUserCert` and `GenerateHostCert`
functions in `lib/auth/auth.go`.

### Disable initiating new connections

Even if a locked-out user is already in a posinteraction of a valid Teleport certificate,
they should be prevented from initiating a new session. This is covered by
adding a check to `auth.Authorize`.

This restriction touches _all_ the access proxies/servers supported by Teleport: SSH, k8s, app and DB.  A lock in force will also block initiating sessions with the proxy web UI and sending requests to the Auth API (both gRPC and HTTP/JSON).

### Terminate existing connections

Terminating an established session/connection due to a (freshly created) lock
is similar to terminating an existing session due to certificate expiry.  The
support for the latter is covered by `srv.Monitor`. To add support for locks,
`srv.Monitor` will be passed in a reference to `LockWatcher` associated with
the Teleport instance.

The developed logic should work with every protocol that features "live sessions"
in the proper sense and that makes use of `srv.Monitor`: SSH, k8s and DB.

### Replication to leaf clusters

`Lock` resources are replicated from the root cluster to leaf clusters
in a similar manner to how CAs are shared between trusted clusters (the
`periodicUpdateCertAuthorities` routine), i.e. by periodically invoking
a leaf cluster's API to replace the locks associated with the root cluster
with the most recent set of locks active at the root cluster.

`Lock` resources received from the root cluster should be stored under
a separate backend key namespace:
```
/locks/<clusterName>/<lockName>
```

### Scenarios

#### Creating a permanent lock

```
$ tctl lock --user=foo@example.com --message="Suspicious activity."
Created a lock with name "dc7cee9d-fe5e-4534-a90d-db770f0234a1".
```

This locks out `foo@example.com` without automatic expiration.
The lock can be lifted by `tctl rm lock/dc7cee9d-fe5e-4534-a90d-db770f0234a1`.

The above locking command would be equivalent to
```sh
tctl create <<EOF
kind: lock
metadata:
  name: dc7cee9d-fe5e-4534-a90d-db770f0234a1
spec:
  message: "Suspicious activity."
  target:
    user: foo@example.com
version: v2
EOF
```

The showed YAML would also correspond to the output of `tctl get lock/dc7cee9d-fe5e-4534-a90d-db770f0234a1`.

#### Creating a lock with expiry

```
$ tctl lock --role=developers --message="Cluster maintenance." --ttl=10h
Created a lock with name "dc7cee9d-fe5e-4534-a90d-db770f0234a1".
```

This locks out users with the role `developers` for the next 10 hours.

Assuming the time at which the command is issued on 2021-06-14 at 12:27 UTC,
the above locking command would be equivalent to both
```sh
tctl create <<EOF
kind: lock
metadata:
  name: dc7cee9d-fe5e-4534-a90d-db770f0234a1
spec:
  target:
    role: developers
  message: "Cluster maintenance."
  expires: "2021-06-14T22:27:00Z"   # RFC3339
version: v2
EOF
```
and
```sh
tctl lock --role=developers --message="Cluster maintenance." --expires="2021-06-14T22:27:00Z"
```

#### Generation of new user certificates prevented

Attempts to generate a new user certificate (`tsh login`, `tctl auth sign`)
while a lock targeting role `developers` is in force will result in the
following error:

```
ERROR: lock targeting Role:"developers" is in force: Cluster maintenance.
```

#### SSH session with an already issued certificate prohibited

```
$ tsh ssh graviton-node
ERROR: ssh: rejected: administratively prohibited (lock targeting User:"andrej" is in force: Malicious behaviour.)
```

#### Live SSH session terminated

An informational message is printed before the connection is terminated:

```
Lock targeting Role:"developers" is in force: Cluster maintenance.
the connection was closed on the remote side on  15 Jun 21 10:43 CEST
```

#### Locking out a node (Deprecated)

`tctl lock --node` was deprecated in favor of `tctl lock --server-id` in Teleport 13.x.x,
12.4.x and 11.3.x. The `--node` flag is still supported for backward compatibility but
it is recommended to use `--server-id` instead.

```
$ tctl lock --node=node-uuid
```

#### Locking out any agent

`tctl lock --server-id` was introduced in Teleport 13.x.x,
12.4.x and 11.3.x and allows locking out any agent (SSH, Kubernetes, Database, etc)
by specifying the agent's UUID. If the agent runs multiple services, all of them
will be locked out.

```
$ tctl lock --server-id=agent-uuid
```

#### Locking out an MFA device ID

```
$ tctl lock --mfa-device=device-uuid
```

#### Locking out a Trusted device ID

```
$ tctl lock --device=device-uuid
```

#### Locking out a Windows Desktop

```
$ tctl lock --windows-desktop=windows-desktop-name
```
