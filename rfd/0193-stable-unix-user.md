---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: draft
---

# RFD 0193 - Stable UNIX user UIDs

## Required Approvers

* Engineering: @rosstimothy && @eriktate

## What

Add a way for the control plane to generate and store "stable" UIDs to be used for automatically provisioned UNIX users across all Teleport SSH servers.

## Why

To support interoperability with tools that rely on UIDs to identity users across different machines running the Teleport SSH server.

## Goal

After the appropriate setting is enabled in the control plane, all compliant (i.e. up to date) Teleport SSH nodes will query the control plane to know which UID to use when attempting to provision a host user if a Teleport user logs into the machine over SSH with a username that doesn't currently exist as a host user on the machine, with a roleset that allows for host user creation in "keep" mode for the specified machine and username, and with no `host_user_uid` trait.

Using the returned UID will be a requirement for both the user and for its primary group: if another host user has the same UID, or a group has the same GID - and thus user creation will fail - the login will also fail. If the host user already exists on the machine, just like the current behavior, the login will just proceed with the existing user. The `host_user_uid` trait, if set, will take priority over the stable UID - effectively, this new feature acts as a fallback strategy to define `host_user_uid` and `host_user_gid`.

If the user creation mode is "insecure-drop", the UID will be chosen following the existing logic and will not be fetched and persisted from the control plane. The same interplay between the current managed host users (in "drop" and "keep" mode) and static host users will apply to managed host users that have a stable UID defined at the cluster level - it's important to keep in mind that existing users' UIDs will never be reassigned no matter which subsystem has control over the user, however, so it's not advisable to have an overlap between static host users' usernames and usernames allowed to autoprovision, or between fixed UIDs in user traits or in static users and the stable UID range. Since it's already possible to have overlaps between UID traits and static users' UIDs, and it's technically possible to carefully configure a cluster to have disjoint sets of hosts where only one of the two features will apply - and also keeping in mind that our cluster state storage is ill suited to maintaining complex invariants - we will not enforce separation between existing or new static host users and the new stable UID range.

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

The list of persisted usernames and UIDs will be made readable through `tctl stable-unix-users ls`. The list will be displayed in a table by default, or in json format if the appropriate option is passed.

## Backwards and forwards compatibility

Teleport SSH servers that are not aware of this feature will ignore it and proceed to provision users with the current behavior; this is the best we can hope for. A missing `spec.stable_unix_user_config` has the same behavior as `spec.stable_unix_user_config.enabled: false` so a updated SSH server connected to an older control plane will treat the feature as disabled.

During a rolling control plane upgrade it's technically possible for a SSH server to read the feature as enabled from an up-to-date Auth Service agent and then fail to obtain a UID from a not-yet-updated one; outside of very contrived scenarios, the window of time for this to happen is so short that we can treat such a failure to obtain a UID just like every other failure to obtain one - not letting the SSH session start.

Since the SSH server is only going to look at the single boolean flag to know whether or not to ask the control plane for a UID to use, we're free to evolve the scheme to change the strategy used to pick UIDs as an Auth Service-only change (say, to support multiple disjoint UID ranges).

## Internals

The Teleport SSH agent will use a new `teleport.stableunixusers.v1.StableUNIXUsersService/ObtainUIDForUsername` rpc to query the UID for any new username; the auth server will generate and persist a UID if one hasn't been set for the given username, or read the already persisted one, and then it will return it to the agent. A `ListStableUNIXUsers` rpc will allow for paginated listing of the persisted usernames and UIDs.

```proto
service StableUNIXUsersService {
  rpc ObtainUIDForUsername(ObtainUIDForUsernameRequest) returns (ObtainUIDForUsernameResponse) {
    option idempotency_level = IDEMPOTENT;
  }

  rpc ListStableUNIXUsers(ListStableUNIXUsersRequest) returns (ListStableUNIXUsersResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}

message ObtainUIDForUsernameRequest {
  string username = 1;
}

message ObtainUIDForUsernameResponse {
  int32 uid = 1;
}

message ListStableUNIXUsersRequest {
  int32 page_size = 1;
  string page_token = 2;
}

message StableUNIXUser {
  string username = 1;
  int32 uid = 2;
}

message ListStableUNIXUsersResponse {
  repeated StableUNIXUser stable_unix_users = 1;
  string next_page_token = 2;
}
```

In the initial implementation there will be no tunables other than the range of UIDs available for use and the `enabled` flag. It is not planned to ever allow for deleting a UID after it's been assigned, but in the future we could add the ability to clear the assigned username for a given UID while still leaving it "occupied".

The cluster state storage will contain a bidirectional mapping of usernames and UIDs, consisting of two items per username, one keyed by username at `/stable_unix_users/by_username/<hex username>` containing the UID and one keyed by UID at `/stable_unix_users/by_uid/<encoded uid>` containing the username. The UID encoding consists of transforming the UID by subtracting it to the maximum 32-bit integer (`0x7fff_ffff`), then writing it in hexadecimal big-endian. This allows for reading the occupied UIDs in large-to-small order through a backend range read in ascending order (since we currently can't scan the backend backwards).

If reading `by_username/<hex username>` succeeds, the UID stored in it will be returned (even if outside of the currently defined UID range); otherwise, the next available free UID is searched by issuing a range read with size one from the end to the beginning of the configured range. Such a read will return the biggest UID in use in the range - or nothing, if no UIDs are in range - letting us pick a free UID. Note that this, at least in the initial implementation, will only allow for using the contiguous range of unused UIDs at the end of the configured range. Changes in the UID range configuration might not result in actually using the entire range. This limitation stems from ease of implementation and might be lifted in the future, if such a need arises (as a workaround, the available range can be shrunk to precede any allocated contiguous range of UIDs).

Seeing as we are not going to change UID allocations, the auth server will use a time-based cache with a short TTL (30 seconds) for the very happy path (in which the username already has an assigned UID), to avoid bursts of backend reads as a result of a new username logging into several different servers for the first time in quick succession (with a `tsh` multi-host command or with something like Ansible). Since there's no expectation that any given value will be heavily read (since, in theory, each host will read each user at most once) and every write will require hard reads and atomic operations, we don't expect to need to replicate the data in the auth cache or to support watching, at least in the first implementation.

## Security and auditability

The `ObtainUIDForUsername` rpc will require `create`+`read` permissions on the `stable_unix_user` pseudo-kind, which will be granted to the `Node` builtin role. The `ListStableUNIXUsers` rpc will require `read`+`list` permissions, which can be assigned to a user or bot for inspection and automation, and will be granted to the preset `auditor` role.

Assuming that the UID range chosen by the cluster admin doesn't implicitly grant extra permissions to the provisioned users somehow, there should be no difference in behavior on the machines running the Teleport SSH server, other than picking a UID rather than letting the system tooling pick an available one from the local user database.

As a side effect of `ObtainUIDForUsername`, after creating a new username/UID association, we will emit an audit log event (`stable_unix_user.create`) containing the username, the associated UID and the host ID of the SSH server that issued the request.

## Observability

The `grpc_server_handled_total` prom metric will keep track of uses and failures of the `ObtainUIDForUsername` RPC; depending on the specifics of the implementation we might add metrics to count LRU cache hits and misses, internal retries caused by contention, and "slow path" UID allocation fallbacks (hopefully caused by configuration changes and not bugs).
