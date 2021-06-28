---
authors: Andrej Tokarčík (andrej@goteleport.com)
state: draft
---

# RFD XY - Locking

## What

This RFD provides a locking mechanism to restrict access to a Teleport
environment.  When such a lock is in force, all interactions involving
identities described by the lock are either terminated or prevented.

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
    // Target describes the identities that the lock applies to.
    LockTarget Target;

    // Message is the message displayed to locked-out users.
    string Message;

    // Expires if set specifies TTL for the lock.
    google.protobuf.Timestamp Expires;
}

// LockTarget lists the attributes all of which (when set)
// must match for the lock to disable their interactions.
// The attributes are interpreted/matched qua simple names,
// with no support for wildcards or regular expressions.
message LockTarget {
    // User specifies the name of a Teleport user.
    string User;

    // Role specifies the name of an RBAC role known to the root cluster.
    // In remote clusters, this constraint is evaluated before translating to local roles.
    string Role;

    // Cluster specifies the name of a leaf Teleport cluster.
    // This prevents the root cluster's users from connecting to nodes in the leaf cluster
    // but does not prevent the leaf cluster from heartbeating back to the root cluster.
    string Cluster;

    // Login specifies the name of a local UNIX user.
    string Login;

    // Node specifies the name or UUID of a node.
    // A matching node is also unregistered and prevented from registering.
    string Node;

    // MFADevice specifies the ID of an MFA device recorded in a user certificate.
    string MFADevice;
}
```

In most cases `LockTarget` should list just a single attribute.
(It would be preferable to define it as a `oneof`, however setting a custom
`jsontag` for `oneof` is [not supported by `gogoproto`](https://github.com/gogo/protobuf/issues/623).)

Note that `Lock` is not a singleton resource: when there are multiple
`Lock`s stored (and in force), it suffices for an interaction to be matched
by any one of the locks to be terminated/disabled.

#### Relation to `User.Status`

There already exists a field of `User` resource that is used to
capture a user lock in connection with failed Web UI login attempts.

This `Status` field and its `LoginStatus` definition are superseded by
`Lock`.  All of its use cases should be converted to `Lock`.

The `Lock` approach allows to specify locks for entities that are only
yet to exist or exist merely transiently (such as SSO user objects).  It could
also help with alleviating the load associated with caching `User` resources.

#### `tctl` support

`Lock` resources can be managed using `tctl [get|create|rm]`.  In this way,
it is possible to update or remove a lock after it has been created.

There will be a special `tctl lock` helper provided, to facilitate creation
of new `Lock`s, see Scenarios below.

### Propagation within a Teleport cluster

For the purposes of lock detection and evaluation, the `Lock` resource will be propagated using the cache system to all Teleport roles/services.

An `auth.AccessPoint` wrapper will be introduced that makes the getters return a `trace.ConnectionProblem` for stale data. The wrapper will be parametrized by a duration that defines the maximum failure period after which the data are considered stale.

`LockTarget` will be convertible into a filter for `types.WatchKind`, allowing
the creation of watchers that match specific contexts. The `GetLocks` method
should also accept `LockTarget`s as filters.

### Fallback mode

If the "strict" cache returns an error indicating stale data, there is a decision to be made about whether to rely on the last known locks. There will be two levels on which to determine this decision.

1. If a lockable transaction involves a user and the user's certificate has an extension named `teleport-lock-fallback`, the transaction is locked out based on the extension's value which comes from the user's RBAC role:

```
spec:
   options:
       # In strict mode, if the locks are not up to date, all matching interactions will be terminated.
       # In best_effort mode, the most recent view of the locks will remain to be used.
       lock: [strict|best_effort]
```

2. A `LockFallback` field is added to the `ClusterAuthPreference` resource. The new field configures the cluster-wide fallback mode that applies when a more specific hint is not available. It defaults to `strict`.

### Disable generating new certificates

No new user or host certificates matching a lock target should be generated
while the lock is in force.  An audit event should be emitted upon such
an attempt.

A new lock check will be added to the `generateUserCert` and `GenerateHostCert`
functions in `lib/auth/auth.go`.

### Disable initiating new session

Even if a locked-out user is already in a posinteraction of a valid Teleport certificate,
they should be prevented from initiating a new session.

This should be implemented so that it touches _all_ the access proxies
supported by Teleport: SSH, k8s, app and DB.

### Terminate existing sessions

Terminating an established session/connection due to a (freshly created) lock
is similar to terminating an existing session due to certificate expiry.  The
support for the latter is covered by `srv.Monitor` and its `Start` routine.

In order to make `srv.Monitor` keep track of all the `Lock`s without
periodically polling the backend, two new fields are added to `srv.MonitorConfig`:
+ `StoredLocks`: a slice of `Lock`s known at the time of calling
  `srv.NewMonitor`;
+ `LockWatcher`: a `types.Watcher` detecting additional puts or deletes
  of `Lock`s pertaining to the current session.

`LockWatcher` will be similar to `services.ProxyWatcher` in that it should
reload automatically after a failure.  If a tolerance interval is exceeded
while performing such reloads, the fallback mode described above is employed
until a healthy connection is reestablished.

The developed logic should work with every protocol that makes use of
`srv.Monitor`: SSH, k8s and DB.

### Replication to leaf clusters

`Lock` resources are replicated from the root cluster to leaf clusters
in a similar manner to how CAs are shared between trusted clusters.

The goal should be achieved by introducing a routine similar to
`periodicUpdateCertAuthorities`. However instead of polling the backend (with
the default period of 10 minutes defined in `defaults.LowResPollingPeriod`)
a `types.Watcher`-based algorithm should be preferred.

To be able to distinguish locks received from the root cluster,
the root cluster should send the `Lock` resource a resource label of the
form:
```
teleport.dev/replicated-from: <root-cluster-name>
```

A tolerance interval similar to the case of intra-cluster propagation should be used.
When a leaf cluster's reverse tunnel connection exhibits intolerable failures,
the leaf cluster's fallback mode will be enforced with respect to the connections
authenticated with a user certificate issued by the root cluster's CA.

### Scenarios

#### Creating a permanent lock

```
$ tctl lock --user=foo@example.com --message="Suspicious activity."
Created a lock with ID "dc7cee9d-fe5e-4534-a90d-db770f0234a1".
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
$ tctl lock --role=developers --message="Cluster maintenance." --expires-in=10h
Created a lock with ID "dc7cee9d-fe5e-4534-a90d-db770f0234a1".
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
ERROR: lock targeting role "developers" is in force (Cluster maintenance.)
```

#### Live SSH session terminated

An informational message is printed before the connection is terminated:

```
Lock targeting role "developers" is in force: Cluster maintenance.
the connection was closed on the remote side on  15 Jun 21 10:43 CEST
```

#### Locking out a node

```
$ tctl lock --node=hostname
```

#### Locking out an MFA device ID

```
$ tctl lock --mfa-device=id
```
