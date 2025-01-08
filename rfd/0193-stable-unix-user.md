---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: draft
---

# RFD 0193 - Stable UNIX user UIDs

## Required Approvers

* Engineering: rosstimothy && eriktate

## What

Add a way for the control plane to generate and store "stable" UIDs to be used for automatically provisioned UNIX users across all Teleport SSH servers.

## Why

To support interoperability with tools that rely on UIDs to identity users across different machines running the Teleport SSH server.

## Goal

After the appropriate setting is enabled in the control plane, all compliant (i.e. up to date) Teleport SSH nodes will query the control plane to know which UID to use when attempting to provision a host user if a Teleport user logs into the machine over SSH with a username that doesn't currently exist as a host user on the machine, with a roleset that allows for host user creation in "keep" mode for the specified machine and username, and with no `host_user_uid` trait. Using the returned UID will be a requirement for both the user and for its primary group: if another host user has the same UID, or a group has the same GID - and thus user creation will fail - the login will also fail. If the host user already exists on the machine, just like the current behavior, the login will just proceed with the existing user. The `host_user_uid` trait, if set, will take priority over the stable UID.

## UX

The `cluster_auth_preference` configuration singleton will grow a new field `spec.stable_unix_user_config`:

```yaml
kind: cluster_auth_preference
version: v2
metadata:
  labels:
    teleport.dev/origin: dynamic
  name: cluster-auth-preference
  revision: 8ac8cd36-7f80-452b-8b77-147a5588f25f
spec:
  # ...
  stable_unix_user_config:
    enabled: true
    first_uid: 7000001
    last_uid: 7019999
```

Teleport SSH servers will check the `enabled` field to know if the feature is enabled, and - if so - they will query the auth server for the UID to use through a new rpc whenever they need to provision a new host user in "keep" mode with no otherwise defined UID. In the initial implementation, provisioned host groups other than the primary group will be generated according to the default system behavior.

## Backwards and forwards compatibility

Teleport SSH servers that are not aware of this feature will ignore it and proceed to provision users with the current behavior; this is the best we can hope for. A missing `spec.stable_unix_user_config` has the same behavior as `spec.stable_unix_user_config.enabled: false` so a updated SSH server connected to an older control plane will treat the feature as disabled.

During a rolling control plane upgrade it's technically possible for a SSH server to read the feature as enabled from an up-to-date Auth Service agent and then fail to obtain a UID from a not-yet-updated one; outside of very contrived scenarios, the window of time for this to happen is so short that we can treat such a failure to obtain a UID just like every other failure to obtain one - not letting the SSH session start.

Since the SSH server is only going to look at the single boolean flag to know whether or not to ask the control plane for a UID to use, we're free to evolve the scheme to change the strategy used to pick UIDs as an Auth Service-only change (say, to support multiple disjoint UID ranges).

## Internals

The Teleport SSH agent will use the new `teleport.userprovisioning.v2.StableUNIXUsersService/ObtainUIDForUsername` rpc to query the UID for any new username; the auth server will generate and persist a UID if one hasn't been set for the given username, or read the already persisted one, and then it will return it to the agent.

```proto
service StableUNIXUsersService {
  rpc ObtainUIDForUsername(ObtainUIDForUsernameRequest) returns (ObtainUIDForUsernameResponse) {
    option idempotency_level = IDEMPOTENT;
  }
}

message ObtainUIDForUsernameRequest {
  string username = 1;
}

message ObtainUIDForUsernameResponse {
  int32 uid = 2;
}
```

In the initial implementation there will be no way to read or manage the list of users and no tunables other than the range of UIDs available for use and the `enabled` flag. It is not planned to ever allow for deleting a UID after it's been assigned, but in the future we could add the ability to clear the assigned username for a given UID while still leaving it occupied.

The cluster state storage will contain a bidirectional mapping of usernames and UIDs, consisting of two items per username, one keyed by username at `/stable_unix_user/by_username/<hex username>` containing the UID and one keyed by UID at `/stable_unix_user/by_uid/<uid as 8 hex digits>` containing the username (to allow for ranged queries in numerical order), as well as a "hint" for the next available UID at `/stable_unix_user/next_uid_hint`.

If reading `by_username/<username>` succeeds, the UID stored in it will be returned (even if outside of the currently defined UID range); otherwise, if the `next_uid_hint` is in the UID range, the auth will attempt to atomically create `by_username/<username>` and `by_uid/<uid>`, update `next_uid_hint`, and assert that `cluster_auth_preference` hasn't changed.

If the operation succeeds, the UID is returned, otherwise the operation is tried again, after some checks: if the UID for the username is still missing, the CAP (after a hard read from the backend) hasn't changed and the `next_uid_hint` hasn't changed (which we have to check separately because a conditional failure of an atomic operation unfortunately doesn't return details about which check has failed), it means that we might have ran out of contiguous space in the UID range. If this is the case, or if `next_uid_hint` is missing, or something else has gone wrong, the Auth server will scan the `by_uid` key range from the `first_uid` to the `last_uid` to find the first available unassigned UID - if `next_uid_hint` is present and in range, the search can start with `next_uid_hint` and then wrap around the end of the range to the beginning. After finding a usable UID, the auth will then proceeed to try the atomic write of `by_username/<username>` and `by_uid/<uid>` again, still asserting the revision of `cluster_auth_preference` and either creating, updating, deleting, or asserting the nonexistence of `next_uid_hint`, depending on whether or not the scan has revealed a second unallocated UID. This somewhat abstruse logic is needed to sanely handle changes in the valid UID range while UIDs have been and are actively being persisted.

Seeing as we are not going to change UID allocations, the auth server will use a simple LRU or time-based cache for the very happy path (in which the username already has an assigned UID), to avoid bursts of backend reads as a result of a new username logging into several different servers for the first time in quick succession (with a `tsh` multi-host command or with something like Ansible). Since there's no expectation that any given value will be heavily read (since, in theory, each host will read each user at most once) and every write will require hard reads and atomic operations, we don't expect to need to replicate the data in the auth cache or to support watching, at least in the first implementation.

## Security and auditability

The `ObtainUIDForUsername` rpc will require `create`+`read` permissions on the `stable_unix_user` pseudo-kind, which will be granted to the `Node` builtin role. Future RPCs might include point and range reads for the mapping, which should require `read` and `read`+`list` permissions respectively, which can be assigned to a user or bot for inspection and automation.

Assuming that the UID range chosen by the cluster admin doesn't implicitly grant extra permissions to the provisioned users somehow, there should be no difference in behavior on the machines running the Teleport SSH server, other than picking a UID rather than letting the system tooling pick an available one from the local user database.

As a side effect of `ObtainUIDForUsername` we will emit one of two new audit log events, `stable_unix_user.create` or `stable_host_user.read`, depending on whether the username already had an associated stable UID or not; the events will be identical other than the event name and code, containing the username, the associated UID and the host ID of the SSH server that issued the request.

## Observability

The `grpc_server_handled_total` prom metric will keep track of uses and failures of the `ObtainUIDForUsername` RPC; depending on the specifics of the implementation we might add metrics to count LRU cache hits and misses, internal retries caused by contention, and "slow path" UID allocation fallbacks (hopefully caused by configuration changes and not bugs).
