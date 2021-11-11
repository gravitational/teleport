---
authors: Joel Wejdenstål (jwejdenstal@goteleport.com)
state: implemented
---

# RFD 43 - Shared sessions with observers for Kubernetes Access

## What

Implement joint observer support for Kubernetes Access with support for configurable conditions similar to those of [RFD 26](https://github.com/gravitational/teleport/blob/2fd6a88800604342bfa6277060b056d8bf0cbfb2/rfd/0026-custom-approval-conditions.md).
Also support defining conditions for required observers in order to initiate and maintain a session.

## Why

Heavily regulated and security critical industries require that one or more observers with a certain role
are present in Kubernetes Access sessions and viewing it live in order to guarantee that
operator does not perform any mistakes or acts of malice.

Such observers need to have the power to terminate a session immediately should anything go wrong.

To suit everyone this will need a more detailed configuration model based on rules
that can be used to define observers, their powers and when and in what capacity they are required.

## Details

### Multiparty sessions

SSH sessions via TSH currently have rich support for sessions with multiple users at once.
This concept is to be extended to Kubernetes Access which will allow us to build additional features on top.

Multiparty sessions shall be implemented by modifying the k8s request proxy forwarder in the `kubernetes_service`. This
approach was chosen as it is a hub that sessions pass through which makes it optimal for multiplexing.

An approach using multiplexing in the `proxy_service` layer was considered but was ultimately determined to be more complicated
due to the fact that proxies don't handle the final session traffic hop when using Kubernetes Access.

It will work by adding a multiplexing layer inside the forwarder that similar to the current session recording
functionality, but instead this multiplexes outputs to the session initiator and all observers
but only streams back input from the initiator.

#### Session observers

A core feature we need to support is required observers. This will allow cluster administrators to configure
policies that require certain Teleport users of a certain role to be actively monitoring the session.

This feature is useful in security critical environments where multiple people need to witness every action
in the event of severe error or malice and have the ability to halt any erroneous or malicious action.

#### Session states

By default, a `tsh kube exec` and `kubectl attach` request will go through as usual if no policies are defined. If a policy like the one above is defined the session will be put in a pending state
until the required viewers have joined.

Sessions can have 3 possible states:

- `PENDING`\
  When a session is in a `PENDING` state, the connection to the pod from the proxy has not yet started
  and all users are shown a default message informing them that the session is pending, current participants
  and who else is required for the session to start.
- `RUNNING`\
A `RUNNING` session behaves like a normal multiparty session. `stdout`, `stdin` and `stdout` are mapped as usual
  and the pod can be interacted with.
- `TERMINATED`\
  A session becomes `TERMINATED` once the shell spawned inside the pod quits or is forcefully terminated by one of the session participants.

All sessions begin in the `PENDING` state and can change states based on the following transitions:

##### Transition 1: `PENDING -> RUNNING`

When the requirements for present viewers laid out in the role policy are fulfilled,
the session transitions to a `RUNNING` state. This involves initiating the connection to the pod
and setting up the shell. Finally, all clients are multiplexed
onto the correct streams as described previously.

Only the session initiator is able to make input, observers are not connected to the input stream
and may only view stdout/stderr and terminate the session.

##### Transition 2: `RUNNING -> TERMINATED`

When the shell process created on the pod is terminated, the session transitions to a `TERMINATED` state and all clients
are disconnected as per standard `kubectl` behaviour.

##### Transition 3: `RUNNING -> TERMINATED`

Session observers that are present may at any point decide to forcefully terminate the session.
This will instantly disconnect input and output streams to prevent further communication. Once this is done
the Kubernetes proxy will send a termination request to the pod session to request it be stopped.

##### Transition 4: `RUNNING -> PENDING`

If an observer disconnects from the session in a way that causes the policy for required observers to suddenly not be fulfilled,
the session will transition back to a `PENDING` state. In this state, input and output streams are disconnected, preventing any further action.

Here, the connection is frozen for a configurable amount of time as a sort of grace period.

##### Transition 5: `PENDING -> TERMINATED`

After a grace period has elapsed in a session in a session that previously was in a `RUNNING`
state, the session is automatically terminated. This can be cancelled by having the required observers
join back in which transitions the session back to `RUNNING`.

##### Transition 6: `PENDING -> TERMINATED`

Any participant of the session can terminate the session in the `PENDING` state.
This will simply mark the session as terminated and disconnect the participants as no
connection to the pod exists at this time.

#### UI/UX

The initial implementation of multiparty sessions on Kubernetes access will only be supported via CLI access for implementation simplicity.

Terminating the `kubectl` process that started the session terminates the session. Terminating an observer `tsh` process
disconnects the observer from the session and applies relevant state transitions if any.

Terminating the session from a observer `tsh` instance can be done with the key combination `CTRL-T`

##### Session creation

Session creation can happen with the existing flow using `kubectl exec`
but the wrapper command `tsh kube exec --invite=bob@example.com,eve@foo.net --reason="Need to fix this pod" -- database_pod -- /bin/bash`. This subcommand allows you to invite one or more accounts which will receive a notification saying they are invited. An arbitrary string may also be provided as a reason
for the session invite, it could for example be used to say what the purpose of the session is.

##### Session join

`kubectl` itself has no concept of multiparty sessions. This means that we cannot easily use
its built-in facilities for support session joining.

To make this process easier for the user. I propose extending the current `tsh join` command
to also work for Kubernetes access in the form of `tsh kube join <session-id>`. This attaches
to an ongoing session and displays stdout/stderr.

##### MFA tap

If the standard `per_session_mfa` option is enabled for a role then MFA tap input via Yubikey or other is required for the participant to be considered active.
This requirement is on an interval of 1 minute. When there is 15 seconds left, an alert is printed to the console.

```
Teleport >> Please tap your MFA key within 15 seconds.
```

If tap is made after the alert, the follwing message is shown:

```
Teleport >> MFA tap received.
```

##### Example

This example illustrates how a group 3 users of which Alice is the initiator and Eve and Ben are two observers
start a multiparty session. Below is a a series of events that happen that include what each user sees and what
they do.

- Alice initiates an interactive session to a pod: `tsh kube exec --invite=ben@foo.net,eve@foo.net,alice@foo.net -- database_pod -- /bin/bash`
- Alice sees:
```
Creating session with uuid <example-uuid>...
This session requires moderator. Waiting for others to join:
- role: auditor-role x2
```
- Eve joins the session with `tsh kube join <example-uuid>` and sees:
```
Please tap MFA key to continue...
```
- Eve taps MFA
- Alice and Eve sees:
```
Creating session with uuid <example-uuid>...
This session requires moderator. Waiting for others to join:
- role: auditor-role x1
Events:
- User Eve joined the session.
```
- Ben joins the session with `tsh kube join <example-uuid>` and sees:
```
Please tap MFA key to continue...
```
- Ben taps MFA
- Alice, Eve and Ben sees
```
Creating session with uuid <example-uuid>...
Session starting...
Events:
- User Eve joined the session.
- User Ben joined the session
```
- The connection to the pod is made and each the session turns into a normal shell.

#### Session invites and notifications

Shared sessions for Kubernetes access will have support for participant invites and notifications.
Ongoing sessions are tracked and can be listed to make it easier to find and join them.

Ongoing sessions you have access to view can be listed with `tsh kube sessions ls`.
This easily allows eligible participants to find and join a session waiting for participants easily

When the `--invite` flag is used with `tsh kube exec`, the invitees are tracked and included in the
session resource which allows Teleport clients and plugins to detect notify them.

##### Session resource

There currently isn't a general purpose session resource in Teleport that's suitable.
therefore I suggest that this shall be added. This will be initially used for tracking Kubernetes
sessions but is compatible with all current and future session types.

This resource is stored centrally in the backend and is used for storing and tracking metadata of active
sessions. Detailed runtime information needed to join such as the TTY size is stored in memory on the the multiplexing node.

This effectively replaces the resource defined [here](https://github.com/gravitational/teleport/blob/master/lib/session/session.go).

```protobuf
// SessionSpecV3 is the specification for a live session.
message SessionSpecV3 {
    // SessionID is unique identifier of this session.
    string SessionID = 1 [ (gogoproto.jsontag) = "session_id,omitempty" ];

    // Namespace is a session namespace, separating sessions from each other.
    string Namespace = 2 [ (gogoproto.jsontag) = "namespace,omitempty" ];

    // Type describes what type of session this is.
    SessionType Type = 3 [ (gogoproto.jsontag) = "type,omitempty" ];

    // State is the current state of this session.
    SessionState State = 4 [ (gogoproto.jsontag) = "state,omitempty" ];

    // Created encodes the time at which the session was registered with the auth
    // server.
    google.protobuf.Timestamp Created = 5 [
        (gogoproto.stdtime) = true,
        (gogoproto.nullable) = false,
        (gogoproto.jsontag) = "created,omitempty"
    ];

    // Expires encodes the time at which this session expires and becomes invalid.
    google.protobuf.Timestamp Expires = 6 [
        (gogoproto.stdtime) = true,
        (gogoproto.nullable) = false,
        (gogoproto.jsontag) = "expires,omitempty"
    ];

    // AttachedData is arbitrary attached JSON serialized metadata.
    string AttachedData = 7 [ (gogoproto.jsontag) = "attached,omitempty" ];

    // Reason is an arbitrary string that may be used to describe the session and/or it's
    // purpose.
    string Reason = 8 [ (gogoproto.jsontag) = "reason,omitempty" ];

    // Invited is a list of invited users, this field is interpreted by different
    // clients on a best-effort basis and used for delivering notifications to invited users.
    repeated string Invited = 9 [ (gogoproto.jsontag) = "invited,omitempty" ];

    // LastActive holds the information about when the session
    // was last active
    google.protobuf.Timestamp LastActive = 10 [
        (gogoproto.stdtime) = true,
        (gogoproto.nullable) = false,
        (gogoproto.jsontag) = "last_active,omitempty"
    ];

    // Hostname is the address of the target this session is connected to.
    string Hostname = 12 [ (gogoproto.jsontag) = "target_hostname,omitempty" ];

    // Address is the address of the target this session is connected to.
    string Address = 13 [ (gogoproto.jsontag) = "target_address,omitempty" ];

    // ClusterName is the name of cluster that this session belongs to.
    string ClusterName = 14 [ (gogoproto.jsontag) = "cluster_name,omitempty" ];

    // Login is the local login/user on the target used by the session.
    string Login = 15 [ (gogoproto.jsontag) = "login,omitempty" ];

    // Participants is a list of session participants.
    repeated Participant Participants = 16 [ (gogoproto.jsontag) = "participants,omitempty" ];
}

// Participant stores information about a participant in the session.
message Participant {
    // ID is a unique UUID of this participant for a given session.
    string ID = 1 [ (gogoproto.jsontag) = "id,omitempty" ];

    // User is the canonical name of the Teleport user controlling this participant.
    string User = 2 [ (gogoproto.jsontag) = "user,omitempty" ];

    // LastActive is the last time this party was active in the session.
    string LastActive = 3 [ (gogoproto.jsontag) = "id,omitempty" ];
}

// SessionType encodes different types of sessions.
enum SessionType {
    // SessionTypeNone is a placeholder variant and isn't valid.
    SessionTypeNone = 0;

    // SessionTypeKubernetes means a session initiated via Kubernetes Access.
    SessionTypeKubernetes = 1;

    // SessionTypeSSH means a standard SSH session initiated via `tsh` or web.
    SessionTypeSSH = 2;
}

// SessionState represents the state of a session.
enum SessionState {
    // Pending variant represents a session that is waiting on participants to fulfill the criteria
    // to start the session.
    SessionStatePending = 0;

    // Running variant represents a session that has had it's criteria for starting
    // fulfilled at least once and has transitioned to a RUNNING state.
    SessionStateRunning = 1;

    // Terminated variant represents a session that is no longer running and due for removal.
    SessionStateTerminated = 2;
}
```

### Configurable Model Proposition

Instead of having fixed fields for specifying values such as required session viewers and roles this
model centers around conditional allow rules and filters. It is implemented as a bi-directional mapping between the role of the session initiator and the roles of the other session participants.

Roles can have `require_session_join` rule under `allow` containing requirements for session participants
before a session may be started with privilege access to nodes that the role provides.

Roles can also have an `join_sessions` rule under `allow` specifying which roles
and session types that that the role grants privileges to join.

We will only initially support the modes `moderator` for Kubernetes Access and `peer` for SSH sessions.
An `observer` mode also exists which only grants access to view but not terminate an ongoing session.

Imagine you have 4 roles:
- `prod-access`
- `senior-dev`
- `customer-db-maintenance`
- `maintenance-observer`

And these requirements:
- `prod-access` should be able to start sessions of any type with either one `senior-dev` observeror two `dev` observers.
- `senior-dev` should be able to start sessions of any type without oversight.
- `customer-db-maintenance` needs oversight by one `maintenance-observer` on `ssh` type sessions.

Then the 4 roles could be defined as follows:

```yaml
kind: role
metadata:
  name: prod-access
spec:
  allow:
    require_session_join:
      - name: Senior dev oversight
        filter: 'contains(observer.roles,"senior-dev")'
        kinds: ['k8s', 'ssh']
        modes: ['moderator']
        count: 1
      - name: Dual dev oversight
        filter: 'contains(observer.roles,"dev")'
        kinds: ['k8s', 'ssh']
        modes: ['moderator']
```

```yaml
kind: role
metadata:
  name: senior-dev
spec:
  allow:
    join_sessions:
      - name: Senior dev oversight
        roles : ['prod-access', 'training']
        kinds: ['k8s', 'ssh', 'db']
        modes: ['moderator']
```

```yaml
kind: role
metadata:
  name: customer-db-maintenance
spec:
  allow:
    require_session_join:
      - name: Maintenance oversight
        filter: 'contains(observer.roles, "maintenance-observer")'
        kinds: ['ssh']
        modes: ['moderator']
        count: 1
```

```yaml
kind: role
metadata:
  name: maintenance-observer
spec:
  allow:
    join_sessions:
      - name: Maintenance oversight
        roles: ['customer-db-*']
        kind: ['*']
        modes: ['moderator']
```

#### Filter specification

A filter determines if a user may act as an approved observer or not.
To facilitate more complex configurations which may be desired we borrow some ideas from the `where` clause used by resource rules.

To make it more workable, the language has been slimmed down significantly to handle this particular use case very well.

##### Functions

- `set["key"]`: Set and array indexing
- `contains(set, item)`: Determines if the set contains the item or not.

##### Provided variables

- `viewer`
```json
{
  "traits": "map<string, []string>",
  "roles": "[]string",
  "name": "string"
}
```

##### Grammar

The grammar and other language is otherwise equal to that of the `where` clauses used by resource rules and the language
used by approval requests, This promotes consistency across the product, reducing confusion.
