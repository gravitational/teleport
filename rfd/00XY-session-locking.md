---
authors: Andrej Tokarčík (andrej@goteleport.com)
state: draft
---

# RFD XY - Session Locking

## What

This RFD provides a locking mechanism to restrict access to a Teleport
environment.  When such a session lock is in force:
+ any existing sessions matching the lock's conditions are terminated, and
+ creation of new sessions matching the lock's conditions is prevented.

## Why

Security teams require greater control over a session once it's started in
order to be able to:
+ lock out the team during maintenance windows,
+ terminate access for a user prior to hitting the Teleport certificate's TTL,
+ achieve FedRAMP/NIST compliance.

## Details

### `SessionLock` resource

A new resource named `SessionLock` with the following specification is introduced:

```proto
message SessionLockSpecV2 {
    // Target describes the set of sessions to which the session lock applies.
    SessionLockTarget Target;
    // EffectiveFrom is the time point at which the session lock comes into force.
    google.protobuf.Timestamp EffectiveFrom;
    // Message is the message displayed to locked-out users.
    string Message;
}

// SessionLockTarget lists the attributes of a session all of which (when set)
// must match for the lock to apply to the session.
// The attributes are interpreted/matched qua simple names, with no support for
// wildcards of regular expressions.
message SessionLockTarget {
    // User specifies the name of a Teleport user.
    string User;
    // Role specifies the name of a RBAC role.
    string Role;
    // Cluster specifies the name of a Teleport cluster.
    string Cluster;
    // Login specifies the name of a local UNIX user.
    string Login;
}
```

In most cases `SessionLockTarget` should list just a single attribute.
(It would be preferable to define it as a `oneof`, however setting a custom
`jsontag` for `oneof` is [not supported by `gogoproto`](https://github.com/gogo/protobuf/issues/623).)

Note that `SessionLock` is not a singleton resource: when there are multiple
`SessionLock`s stored (and in force), it suffices for a session to be matched
by any one of the locks to be terminated/disabled.

If the conditions encoded by a set of `SessionLock`s was to be expressed in
a single logical formula, the conditions within `SessionLockTarget` would be
connected with logical conjunction while those of multiple `SessionLock`s would
be connected with logical disjunction.  This is similar to the notion of
_disjunctive normal form_ of propositional logic.  In the ordinary case where
`SessionLockTarget` lists just a single attribute, the formula would reduce to
a simple disjunction of atomic constraints.

#### Expiration & audit

There is no explicit expiry field in `SessionLockSpecV2`.  The common `Metadata.Expires`
field is used instead.

Since the `Metadata.Expires` field is also used by the backends to delete stale
data, this automatically guarantees no expired `SessionLocks` will be returned
by methods like `GetSessionLocks`, which in turn implies only unexpired locks
can ever be presented to the user in summary outputs.

Historical `SessionLock` records can be reconstructed only from the audit log.
Every `SessionLock` create/update/delete should cause an audit event to be
emitted.

#### Relation to `User.Status`

There already exists a field of `User` resource that is used to
capture a user lock in connection with failed Web UI login attempts.

This `Status` field and its `LoginStatus` definition are superseded by
`SessionLock`.  All of its use cases should be converted to `SessionLock`.

The `SessionLock` approach allows to specify locks for entities that are only
yet to exist or exist merely transiently (such as SSO user objects).  It could
also help with alleviating the load associated with caching `User` resources.

#### `tctl` support

`SessionLock` resources can be managed using `tctl [get|create|rm]`.  In this
way, it is possible to update (e.g. delay) or remove a session lock after it
has been created.

There will be a special `tctl sessions lock` helper provided, to facilitate
supplying time information when creating new `SessionLock`s, see Scenarios below.

### Disabling new certificates

No new user certificates matching a session lock target should be generated
while the session lock is in force.  An audit event should be emitted upon such
an attempt.

A new lock check will be added to the `generateUserCert` function
in `lib/auth/auth.go`.

### Terminating existing sessions

Terminating an existing session due to a (freshly created) session lock is
similar to terminating an existing session due to certificate expiry.  The
support for the latter is covered by `srv.Monitor` and its `Start` routine.

In order to make `srv.Monitor` keep track of all the `SessionLock`s without
periodically polling the backend, two new fields are added to `srv.MonitorConfig`:
+ `StoredSessionLocks`: a slice of `SessionLock`s known at the time of calling
  `srv.NewMonitor`;
+ `SessionLockWatcher`: a `types.Watcher` detecting additional puts or deletes
  of `SessionLock`s pertaining to the current session.

The developed logic should work with all the proxies that already make use of
`DisconnectExpiredCert`, i.e. SSH, k8s and DB.

### Replicating to trusted clusters

`SessionLock` resources are replicated from the root cluster to leaf clusters
in a similar manner to how CAs are shared between trusted clusters.

The goal should be achieved by introducing a routine similar to
`periodicUpdateCertAuthorities`. However instead of polling the backend (with
the default period of 10 minutes defined in `defaults.LowResPollingPeriod`)
a `types.Watcher`-based algorithm should be preferred.

To be able to distinguish session locks received from a remote (root) cluster,
the root cluster should send the `SessionLock` resource a resource label of the
form:
```
teleport.dev/replicated-from: <root-cluster-name>
```

### Scenarios

#### Creating a permanent lock

```
$ tctl sessions lock --user=foo@example.com --message="Suspicious activity."
Created a session lock with ID "dc7cee9d-fe5e-4534-a90d-db770f0234a1".
```

This locks out `foo@example.com` without automatic expiration.
The lock can be lifted by `tctl rm lock/dc7cee9d-fe5e-4534-a90d-db770f0234a1`.

The above locking command would be equivalent to
```sh
tctl create <<EOF
kind: session_lock
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
$ tctl sessions lock --role=developers --message="Cluster maintenance." --expires-in=10h
Created a session lock with ID "dc7cee9d-fe5e-4534-a90d-db770f0234a1".
```

This locks out users with the role `developers` for the next 10 hours.

Assuming the time at which the command is issued on 2021-06-14 at 12:27 UTC,
the above locking command would be equivalent to both
```sh
tctl create <<EOF
kind: session_lock
metadata:
  name: dc7cee9d-fe5e-4534-a90d-db770f0234a1
  expires: "2021-06-14T22:27:00Z"   # RFC3339
spec:
  target:
    role: developers
  message: "Cluster maintenance."
version: v2
EOF
```
and
```sh
tctl sessions lock --role=developers --message="Cluster maintenance." --expires="2021-06-14T22:27:00Z"
```

#### Creating a lock with a delayed effect

```
$ tctl sessions lock --cluster=leaf --effective-from="2021-06-14T22:27:00Z"
Created a session lock with ID "dc7cee9d-fe5e-4534-a90d-db770f0234a1".
```

This locks out users with the role `developers` effective from 2021-06-14 22:27 UTC.

The above locking command would be equivalent to
```sh
tctl create <<EOF
kind: session_lock
metadata:
  name: dc7cee9d-fe5e-4534-a90d-db770f0234a1
spec:
  target:
    cluster: leaf
  effective_from: "2021-06-14T22:27:00Z"   # RFC3339
version: v2
EOF
```
and, assuming the time at which the command is issued on 2021-06-14
at 12:27 UTC, to
```sh
tctl sessions lock --cluster=leaf --effective-in=10h
```

#### Generation of new user certificates prevented

Attempts to generate a new user certificate (`tsh login`, `tctl auth sign`)
while a session lock targeting role `developers` is in force will result in the
following error:

```
ERROR: session lock targeting role "developers" is in force
```

#### Live SSH session terminated

Terminated just like when `disconnect_expired_cert` is enabled,
showing just a generic client-specific message, e.g.:

```
the connection was closed on the remote side on  15 Jun 21 10:43 CEST
```
